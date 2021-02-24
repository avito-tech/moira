package notifier

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/notifier/contacts"
	"go.avito.ru/DO/moira/senders"
	"go.avito.ru/DO/moira/silencer"
)

// NotificationPackage represent sending data
type NotificationPackage struct {
	Events     moira.NotificationEvents `json:"events"`
	Trigger    moira.TriggerData        `json:"trigger"`
	Contact    moira.ContactData        `json:"contact"`
	FailCount  int                      `json:"fail_count"`
	Throttled  bool                     `json:"throttled"`
	DontResend bool                     `json:"dont_resend"`
	NeedAck    bool                     `json:"need_ack"`

	OverridingTriggerID string `json:"overriding_trigger_id"`
	OverridingMetric    string `json:"overriding_metric"`
}

func (pkg NotificationPackage) String() string {
	return fmt.Sprintf("package of %d notifications to %s://%s", len(pkg.Events), pkg.Contact.Type, pkg.Contact.Value)
}

func (pkg NotificationPackage) StringFull() string {
	result := fmt.Sprintf("Trigger: %s\n", pkg.Trigger.ID)
	result += fmt.Sprintf("To: %s://%s\n", pkg.Contact.Type, pkg.Contact.Value)
	result += fmt.Sprintf("%d events:", len(pkg.Events))
	for i, event := range pkg.Events {
		result += fmt.Sprintf("  %d: %v\n", i, event)
	}
	return result
}

// Notifier implements notification functionality
type Notifier interface {
	Send(pkg *NotificationPackage, waitGroup *sync.WaitGroup)
	RegisterSender(senderSettings map[string]string, sender moira.Sender) error
	StopSenders()
	GetSenders() map[string]bool
}

// StandardNotifier represent notification functionality
type StandardNotifier struct {
	config          Config
	contactsDecoder *contacts.Decoder
	database        moira.Database
	logger          moira.Logger
	metrics         *metrics.NotifierMetrics
	scheduler       Scheduler
	senders         map[string]chan NotificationPackage
	silencer        *silencer.Silencer
	waitGroup       sync.WaitGroup
}

// NewNotifier is initializer for StandardNotifier
func NewNotifier(database moira.Database, config Config, metrics *metrics.NotifierMetrics) *StandardNotifier {
	silencerWorker := silencer.NewSilencer(database, nil)
	silencerWorker.Start()

	logger := logging.GetLogger("")
	return &StandardNotifier{
		config:          config,
		contactsDecoder: contacts.NewDecoder(database, logger, config.DutyApiToken, config.DutyUrl),
		database:        database,
		logger:          logger,
		senders:         make(map[string]chan NotificationPackage),
		scheduler:       NewScheduler(database, metrics),
		silencer:        silencerWorker,
		metrics:         metrics,
	}
}

// if duty.cc returns an empty response but no error,
// retry `maxDutyTries` times, then give up
const maxDutyTries = 3

// Send is realization of StandardNotifier Send functionality
func (notifier *StandardNotifier) Send(pkg *NotificationPackage, waitGroup *sync.WaitGroup) {
	ch, found := notifier.senders[pkg.Contact.Type]
	if !found {
		notifier.resend(pkg, fmt.Sprintf("Unknown contact type '%s' [%s]", pkg.Contact.Type, pkg))
		return
	}

	waitGroup.Add(1)
	go func(pkg *NotificationPackage) {
		defer waitGroup.Done()

		logger := logging.GetLogger(pkg.Trigger.ID)
		logger.InfoE(fmt.Sprintf("Start processing package for trigger ID %s", pkg.Trigger.ID), pkg)

		eventsFiltered := make([]moira.NotificationEvent, 0, len(pkg.Events))
		for _, event := range pkg.Events {
			if notifier.silencer.IsMetricSilenced(event.Metric, event.Timestamp) {
				logger.InfoE(fmt.Sprintf("Event is filtered because metric %s is silenced", event.Metric), event)
				continue
			}
			if notifier.silencer.IsTagsSilenced(pkg.Trigger.Tags, event.Timestamp) {
				logger.InfoE("Event is filtered due to tags", event)
				continue
			}
			eventsFiltered = append(eventsFiltered, event)
		}

		if len(eventsFiltered) > 0 {
			pkg.Events = eventsFiltered
			select {
			case ch <- *pkg:
				break
			case <-time.After(notifier.config.SendingTimeout):
				notifier.resend(pkg, fmt.Sprintf("Timeout sending %s", pkg))
				break
			}
		} else {
			logger.InfoE("The whole package is silenced, don't send it", pkg)
		}
	}(pkg)
}

// GetSenders get hash of registered notifier senders
func (notifier *StandardNotifier) GetSenders() map[string]bool {
	hash := make(map[string]bool)
	for key := range notifier.senders {
		hash[key] = true
	}
	return hash
}

func (notifier *StandardNotifier) resend(pkg *NotificationPackage, reason string) {
	if pkg.DontResend {
		return
	}

	notifier.metrics.SendingFailed.Increment()
	if metric, found := notifier.metrics.SendersFailedMetrics.GetMetric(pkg.Contact.Type); found {
		metric.Increment()
	}

	failCount := pkg.FailCount + 1

	logger := logging.GetLogger(pkg.Trigger.ID)
	logger.Error(fmt.Sprintf("Can't send message after %d try: %s. Will retry again", failCount, reason))

	if notifier.scheduler.CalculateBackoff(failCount) > notifier.config.ResendingTimeout {
		logger.Error("Stop resending. Notification interval is timed out")
		return
	}

	for _, event := range pkg.Events {
		next, throttled := notifier.scheduler.GetDeliveryInfo(time.Now(), event, pkg.Throttled, failCount)
		notification := notifier.scheduler.ScheduleNotification(next, throttled, event, pkg.Trigger, pkg.Contact, failCount, pkg.NeedAck)
		if err := notifier.database.AddNotification(notification); err != nil {
			logger.Error(fmt.Sprintf("Failed to save scheduled notification: %v", err))
		}
	}
}

func (notifier *StandardNotifier) run(sender moira.Sender, ch chan NotificationPackage) {
	defer notifier.waitGroup.Done()

	for pkgFetched := range ch {
		go func(pkg NotificationPackage) {
			logger := logging.GetLogger(pkg.Trigger.ID)
			senderWI, inheritance := sender.(moira.SenderWithInheritance)

			logger.InfoE("Sending notification package", map[string]interface{}{
				"inheritance": inheritance,
				"package":     pkg,
			})

			// filtering events depending sender type
			eventsFiltered := make([]moira.NotificationEvent, 0, len(pkg.Events))
			for _, event := range pkg.Events {
				// for inheritance sender, delete events with OverriddenByAncestor == true
				// for non-inheritance sender, delete forced notifications
				// both is done so that users won't receive duplicate notifications
				eventOK := (inheritance && !event.OverriddenByAncestor) || (!inheritance && !event.IsForceSent)
				if eventOK {
					eventsFiltered = append(eventsFiltered, event)
				}
			}

			oldQty, newQty := len(pkg.Events), len(eventsFiltered)
			pkg.Events = eventsFiltered

			if oldQty != newQty {
				logger.InfoE(fmt.Sprintf("Filtered %d event(s)", oldQty-newQty), map[string]interface{}{
					"inheritance": inheritance,
					"package":     pkg,
				})
			}

			if newQty == 0 {
				logger.Info("Package has got empty, drop it")
				return
			}

			// if contact value is macro (duty, deployer) then expand it to the list of actual contacts
			replacements, err := notifier.contactsDecoder.UnwrapContact(&pkg.Contact, pkg.Events)
			if err != nil {
				logger.ErrorE("Failed to unwrap contact", map[string]interface{}{
					"error_message": err.Error(),
					"package":       pkg,
				})
				resend := true

				switch err.(type) {
				case contacts.ErrNobodyOnDuty:
					if pkg.FailCount >= maxDutyTries {
						logger.Error(fmt.Sprintf("Nobody is on duty (%d attempts exceeded), drop package", maxDutyTries))
						resend = false
					}
				case contacts.ErrNoDeployers:
					logger.Error("Package has no deployers, drop it")
					resend = false
				case contacts.ErrGroupIsEmpty:
					logger.Error("Group is empty, drop the package")
					resend = false
				case contacts.ErrNoServiceChannels:
					logger.Error("Package has no service channels, drop it")
					resend = false
				case senders.ErrSendEvents:
					logger.ErrorF("Sender error: %v", err)
					resend = !err.(senders.ErrSendEvents).Fatal
				}

				if resend {
					notifier.resend(&pkg, err.Error())
				}

				return
			}

			// since there might be more than one destination, some of them can be sent successfully while other can fail
			// so only those which failed will be resent
			sendErrors := make(map[int]error)
			for i := 0; i < len(replacements); i++ {
				// replace macro (if contact value isn't macro then replacement will have no effect)
				pkg.Contact.Expiration = replacements[i].Expiration
				pkg.Contact.Value = replacements[i].ValueReplaced

				if pkg.OverridingTriggerID != "" && inheritance {
					err = senderWI.SendEventsWithInheritance(
						pkg.Events,
						pkg.Contact,
						pkg.Trigger,
						pkg.OverridingTriggerID,
						pkg.OverridingMetric,
					)
				} else {
					err = sender.SendEvents(
						pkg.Events,
						pkg.Contact,
						pkg.Trigger,
						pkg.Throttled,
						pkg.NeedAck,
					)
				}

				if err != nil {
					logger.ErrorE("Error while sending package", map[string]interface{}{
						"contact_original": replacements[i].ValueRollback,
						"contact_replaced": replacements[i].ValueReplaced,
						"error_message":    err.Error(),
						"inheritance":      inheritance,
						"stack_trace":      string(debug.Stack()),
						"package":          pkg,
					})
					sendErrors[i] = err
					continue
				}

				if metric, found := notifier.metrics.SendersOkMetrics.GetMetric(pkg.Contact.Type); found {
					metric.Increment()
				}
			}

			for i, err := range sendErrors {
				// replace actual login to original macro and resend this package
				pkg.Contact.Expiration = nil
				pkg.Contact.Value = replacements[i].ValueRollback
				notifier.resend(&pkg, err.Error())
			}
		}(pkgFetched)
	}
}
