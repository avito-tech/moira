package escalations

import (
	"fmt"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/notifier"
)

// FetchEscalationsWorker - check for new notifications and send it using notifier
type FetchEscalationsWorker struct {
	Database moira.Database
	Notifier notifier.Notifier
	tomb     tomb.Tomb
}

// Start is a cycle that fetches scheduled notifications from database
func (w *FetchEscalationsWorker) Start() {
	logger := logging.GetLogger("")
	w.tomb.Go(func() error {
		checkTicker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-w.tomb.Dying():
				logger.Info("Moira Notifier Fetching scheduled escalations stopped")
				return nil
			case <-checkTicker.C:
				if err := w.processScheduledEscalations(); err != nil {
					logger.WarnF("Failed to fetch scheduled escalations: %v", err)
				}
			}
		}
	})
	logger.Info("Moira Notifier Fetching scheduled escalations started")
}

// Stop stops new notifications fetching and wait for finish
func (w *FetchEscalationsWorker) Stop() error {
	w.tomb.Kill(nil)
	return w.tomb.Wait()
}

func (w *FetchEscalationsWorker) processScheduledEscalations() error {
	type triggerMetric struct {
		triggerID, metric string
	}

	escalationEvents, err := w.Database.FetchScheduledEscalationEvents(time.Now().Unix())
	if err != nil {
		return err
	}

	eventsQty := len(escalationEvents)
	notificationPackages := make(map[string]*notifier.NotificationPackage, eventsQty)
	triggersToAck := make(map[triggerMetric]bool, eventsQty)

	for i := 0; i < eventsQty; i++ {
		escalation := escalationEvents[i]
		logger := logging.GetLogger(escalation.Trigger.ID)
		logger.InfoF("processing trigger %s escalation", escalation.Trigger.ID)

		hasEscalations, err := w.Database.MetricHasPendingEscalations(escalation.Trigger.ID, escalation.Event.Metric, true)
		if err != nil {
			return err
		}
		if !hasEscalations {
			logger.DebugF("Skip trigger: %s no pending escalations", escalation.Trigger.ID)
			continue
		}

		if !escalation.IsResolution {
			lastCheck, err := w.Database.GetTriggerLastCheck(escalation.Trigger.ID)
			if err != nil {
				return nil
			}

			// checking whether current escalation is redundant (because metric state has changed to OK)
			if state, ok := lastCheck.Metrics[escalation.Event.Metric]; ok && state.State == moira.OK {
				logger.Info("Escalation trigger last check was ok")
				continue
			}

			// if escalation is up then it will be registered
			if err := w.Database.RegisterProcessedEscalationID(escalation.Escalation.ID, escalation.Event.Metric, escalation.Trigger.ID); err != nil {
				return err
			}

			if escalation.IsFinal {
				triggersToAck[triggerMetric{triggerID: escalation.Trigger.ID, metric: escalation.Event.Metric}] = false
			}
		} else {
			triggersToAck[triggerMetric{triggerID: escalation.Trigger.ID, metric: escalation.Event.Metric}] = true
		}

		contacts, err := w.Database.GetContacts(escalation.Escalation.Contacts)
		if err != nil {
			return err
		}

		for i, contact := range contacts {
			if contact == nil {
				logger.WarnF("Failed to get contact id = \"%s\", escalation id = \"%s\"", escalation.Escalation.Contacts[i], escalation.Escalation.ID)
				continue
			}

			eventContextString := escalation.Event.Context.MustMarshal()
			packageKey := fmt.Sprintf(
				"%s:%s:%s:%s",
				contact.Type, contact.Value,
				escalation.Trigger.ID,
				eventContextString,
			)
			p, found := notificationPackages[packageKey]
			if !found {
				p = &notifier.NotificationPackage{
					Events:    make([]moira.NotificationEvent, 0, len(escalationEvents)),
					Trigger:   escalation.Trigger,
					Contact:   *contact,
					Throttled: false,
					FailCount: 0,
					NeedAck:   true,
				}
			}
			p.Events = append(p.Events, escalation.Event)
			notificationPackages[packageKey] = p
		}
	}

	for triggerMetric, isResolution := range triggersToAck {
		_ = w.Database.AckEscalations(triggerMetric.triggerID, triggerMetric.metric, isResolution)
	}

	var wg sync.WaitGroup
	for _, pkg := range notificationPackages {
		w.Notifier.Send(pkg, &wg)
	}
	wg.Wait()

	return nil
}
