package notifications

import (
	"fmt"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/notifier"
)

// FetchNotificationsWorker - check for new notifications and send it using notifier
type FetchNotificationsWorker struct {
	Logger                     moira.Logger
	Database                   moira.Database
	TriggerInheritanceDatabase moira.TriggerInheritanceDatabase
	Notifier                   notifier.Notifier
	tomb                       tomb.Tomb
}

// Start is a cycle that fetches scheduled notifications from database
func (worker *FetchNotificationsWorker) Start() {
	worker.tomb.Go(func() error {
		checkTicker := time.NewTicker(time.Second)
		for {
			select {
			case <-worker.tomb.Dying():
				worker.Logger.Info("Moira Notifier Fetching scheduled notifications stopped")
				worker.Notifier.StopSenders()
				return nil
			case <-checkTicker.C:
				if err := worker.processScheduledNotifications(); err != nil {
					worker.Logger.WarnF("Failed to fetch scheduled notifications: %v", err)
				}
			}
		}
	})
	worker.Logger.Info("Moira Notifier Fetching scheduled notifications started")
}

// Stop stops new notifications fetching and wait for finish
func (worker *FetchNotificationsWorker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}

func (worker *FetchNotificationsWorker) processScheduledNotifications() error {
	notifications, err := worker.Database.FetchNotifications(time.Now().Unix())
	if err != nil {
		return err
	}

	if globalSettings, err := worker.Database.GetGlobalSettings(); err != nil {
		return err
	} else if globalSettings.Notifications.Disabled {
		worker.Logger.InfoF("%d notifications are discarded because all notifications are disabled", len(notifications))
		return nil
	}

	notificationPackages := make(map[string]*notifier.NotificationPackage)
	for _, notification := range notifications {
		eventContextString := notification.Event.Context.MustMarshal()
		packageKey := fmt.Sprintf("%s:%s:%s:%t:%s",
			notification.Contact.Type, notification.Contact.Value,
			notification.Event.TriggerID, notification.NeedAck,
			eventContextString,
		)
		p, found := notificationPackages[packageKey]
		if !found {
			p = &notifier.NotificationPackage{
				Events:    make([]moira.NotificationEvent, 0, len(notifications)),
				Trigger:   notification.Trigger,
				Contact:   notification.Contact,
				Throttled: notification.Throttled,
				FailCount: notification.SendFail,
				NeedAck:   notification.NeedAck,
			}
		}
		p.Events = append(p.Events, notification.Event)
		notificationPackages[packageKey] = p
	}

	var sendingWG sync.WaitGroup
	for _, pkg := range notificationPackages {
		if len(pkg.Trigger.Parents) > 0 {
			pkgs := worker.processTriggerAncestors(pkg)
			for _, pkg := range pkgs {
				worker.Notifier.Send(pkg, &sendingWG)
			}
		}
		worker.Notifier.Send(pkg, &sendingWG)
	}
	sendingWG.Wait()

	return nil
}

func (worker *FetchNotificationsWorker) processTriggerAncestors(pkg *notifier.NotificationPackage) []*notifier.NotificationPackage {
	type triggerMetric struct {
		triggerID, metric string
	}

	events := make(map[triggerMetric][]moira.NotificationEvent)
	for _, event := range pkg.Events {
		if event.AncestorTriggerID != "" {
			key := triggerMetric{event.AncestorTriggerID, event.AncestorMetric}
			events[key] = append(events[key], event)
		}
	}

	pkgs := make([]*notifier.NotificationPackage, 0, len(events))
	for key, events := range events {
		pkgCopy := *pkg
		newPkg := &(pkgCopy)
		newPkg.OverridingTriggerID = key.triggerID
		newPkg.OverridingMetric = key.metric
		newPkg.Events = events
		newPkg.Contact = moira.ContactData{Type: newPkg.Contact.Type}
		pkgs = append(pkgs, newPkg)
	}
	return pkgs
}
