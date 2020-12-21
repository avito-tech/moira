package events

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/fan"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/notifier"
)

const (
	FanDelay = 10
	// this should be greater than the task TTL used in service-fan
	// (currently 120 seconds)
	FanMaxWait = 150
)

// FetchEventsWorker checks for new events and new notifications based on it
type FetchEventsWorker struct {
	Logger                     moira.Logger
	Database                   moira.Database
	TriggerInheritanceDatabase moira.TriggerInheritanceDatabase
	Scheduler                  notifier.Scheduler
	Metrics                    *metrics.NotifierMetrics
	tomb                       tomb.Tomb
	Fetcher                    func() (moira.NotificationEvents, error)
	Fan                        fan.Client
}

// notificationEscalationFlag stores information about whether subscription which caused this notification has escalations
type notificationEscalationFlag struct {
	HasEscalations bool
	Notification   *moira.ScheduledNotification
}

// Start is a cycle that fetches events from database
func (worker *FetchEventsWorker) Start() {
	worker.tomb.Go(func() error {
		for {
			select {
			case <-worker.tomb.Dying():
				{
					worker.Logger.Info("Moira Notifier Fetching events stopped")
					return nil
				}
			default:
				{
					events, err := worker.Fetcher()
					if err != nil {
						if err != database.ErrNil {
							worker.Metrics.EventsMalformed.Increment()
							worker.Logger.Warn(err.Error())
							time.Sleep(time.Second * 5)
						}
						continue
					}
					for _, event := range events {
						worker.Metrics.EventsReceived.Increment()
						if err := worker.processEvent(event); err != nil {
							worker.Metrics.EventsProcessingFailed.Increment()
							worker.Logger.ErrorF("Failed processEvent. %v", err)
						}
					}
				}
			}
		}
	})
	worker.Logger.Info("Moira Notifier Fetching events started")
}

// Stop stops new event fetching and wait for finish
func (worker *FetchEventsWorker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}

func (worker *FetchEventsWorker) processEvent(event moira.NotificationEvent) error {
	var (
		subscriptions []*moira.SubscriptionData
		tags          []string
		triggerData   moira.TriggerData
	)

	logger := logging.GetLogger(event.TriggerID)
	isTest := event.State == moira.TEST

	if !isTest {
		worker.Logger.DebugF(
			"Processing trigger id %s for metric %s == %f, %s -> %s",
			event.TriggerID, event.Metric, moira.UseFloat64(event.Value),
			event.OldState, event.State,
		)

		trigger, err := worker.Database.GetTrigger(event.TriggerID)
		if err != nil {
			return err
		}
		if len(trigger.Tags) == 0 {
			return fmt.Errorf("No tags found for trigger id %s", event.TriggerID)
		}

		if event.State == moira.OK && len(trigger.Parents) > 0 {
			// OKs should never get delayed
			// so we set DelayedForAncestor (for the code below) but don't actually delay the event
			event.DelayedForAncestor = true
		}

		if !event.DelayedForAncestor {
			var (
				depth       = 0
				err   error = nil
			)

			if worker.TriggerInheritanceDatabase != nil {
				depth, err = worker.TriggerInheritanceDatabase.GetMaxDepthInGraph(event.TriggerID)
			}

			if err != nil {
				logger.ErrorE("Disabling trigger inheritance", map[string]interface{}{
					"TriggerID": event.TriggerID,
					"Error":     err.Error(),
				})
			} else {
				var delay = int64(depth) * 60
				if delay > 0 {
					delay += 60
				}

				if delay > 0 {
					event.DelayedForAncestor = true
					if err := worker.delayEventAndLog(&event, delay); err == nil {
						return nil
					}
				}
			}
		}

		if event.DelayedForAncestor && event.AncestorTriggerID == "" {
			events, ancestors, err := worker.processTriggerAncestors(&event)
			switch {
			case err != nil:
				logger.ErrorF(
					"Could not process ancestors for trigger %s: %v, disabling trigger inheritance!",
					event.TriggerID, err,
				)
			case len(events) > 0:
				logger.InfoE("found ancestors for event", map[string]interface{}{
					"Event":     event,
					"Ancestors": ancestors,
				})
				for _, event := range events {
					if err := worker.processEvent(event); err != nil {
						return err
					}
				}
				return nil
			}
		}

		triggerData = moira.TriggerData{
			ID:         trigger.ID,
			Name:       trigger.Name,
			Desc:       moira.UseString(trigger.Desc),
			Targets:    trigger.Targets,
			Parents:    trigger.Parents,
			WarnValue:  moira.UseFloat64(trigger.WarnValue),
			ErrorValue: moira.UseFloat64(trigger.ErrorValue),
			Tags:       trigger.Tags,
			Dashboard:  trigger.Dashboard,
			Saturation: worker.filterSaturationForEvent(&event, trigger.Saturation),
		}

		if len(triggerData.Saturation) > 0 {
			saturationResult := worker.saturate(&event, &triggerData)
			if saturationResult.Done {
				event = *saturationResult.Event
				triggerData = *saturationResult.TriggerData
			} else {
				return nil
			}
		}

		tags = append(triggerData.Tags, event.GetEventTags()...)
		worker.Logger.DebugF("Getting subscriptions for tags %v", tags)
		subscriptions, err = worker.Database.GetTagsSubscriptions(tags)
		if err != nil {
			return err
		}
	} else {
		sub, err := worker.getNotificationSubscriptions(event)
		if err != nil {
			return err
		}
		subscriptions = []*moira.SubscriptionData{sub}
	}

	notificationSet := make(map[string]notificationEscalationFlag)
	for _, subscription := range subscriptions {
		if subscription == nil {
			worker.Logger.Debug("Subscription is nil")
			continue
		}
		if !isTest {
			if !subscription.Enabled {
				worker.Logger.DebugF("Subscription %s is disabled", subscription.ID)
				continue
			}
			if !subset(subscription.Tags, tags) {
				worker.Logger.DebugF("Subscription %s has extra tags", subscription.ID)
				continue
			}
		}

		next, throttled := worker.Scheduler.GetDeliveryInfo(time.Now(), event, false, 0)
		hasEscalations := len(subscription.Escalations) > 0
		needAck := false
		if hasEscalations {
			if err := worker.Database.MaybeUpdateEscalationsOfSubscription(subscription); err != nil {
				worker.Logger.ErrorF("Failed to update old-style escalations: %v", err)
				continue
			} else if err := worker.Database.AddEscalations(next.Unix(), event, triggerData, subscription.Escalations); err != nil {
				worker.Logger.ErrorF("Failed to save escalations: %v", err)
				continue
			}

			needAck = event.State == moira.ERROR || event.State == moira.WARN
		}

		worker.Logger.DebugF("Processing contact ids %v for subscription %s", subscription.Contacts, subscription.ID)
		for _, contactID := range subscription.Contacts {
			contact, err := worker.Database.GetContact(contactID)
			if err != nil {
				worker.Logger.WarnF("Failed to get contact: %s, skip handling it, error: %v", contactID, err)
				continue
			}
			event.SubscriptionID = &subscription.ID

			notification := worker.Scheduler.ScheduleNotification(next, throttled, event, triggerData, contact, 0, needAck)
			notificationKey := notification.GetKey()

			// notifications with escalations are preferable
			if value, exists := notificationSet[notificationKey]; exists {
				if !hasEscalations || value.HasEscalations {
					// either current notification has no escalations or already existing one has any - no need to replace - skipping current
					worker.Logger.DebugF("Skip duplicated notification for contact %s", notification.Contact)
				} else {
					// current notification has escalations and already existing one has none - replacing with current and skipping existing
					notificationSet[notificationKey] = notificationEscalationFlag{
						HasEscalations: true,
						Notification:   notification,
					}
					worker.Logger.DebugF("Skip duplicated notification for contact %s", value.Notification.Contact)
				}
			} else {
				notificationSet[notificationKey] = notificationEscalationFlag{
					HasEscalations: hasEscalations,
					Notification:   notification,
				}
			}
		}
	}

	// adding unique notifications (with escalations as priority)
	for _, value := range notificationSet {
		if err := worker.Database.AddNotification(value.Notification); err != nil {
			worker.Logger.ErrorF("Failed to save scheduled notification: %s", err)
			logger.ErrorE(fmt.Sprintf("Failed to save scheduled notification: %s", err), value.Notification)
		} else {
			logger.InfoE("Trace added notification", value.Notification)
		}
	}

	return nil
}

// filterSaturationForEvent applies some internal notifier-side logic
// and (probably) reduces the list of saturation methods applicable
// for this moira.NotificationEvent
func (worker *FetchEventsWorker) filterSaturationForEvent(event *moira.NotificationEvent, saturation []moira.Saturation) []moira.Saturation {
	if saturation == nil {
		return nil
	}

	result := make([]moira.Saturation, 0, len(saturation))
	for _, sat := range saturation {
		// don't take screenshots for OK events
		if sat.Type == moira.SatTakeScreen && event.State == moira.OK {
			continue
		}

		result = append(result, sat)
	}

	return result
}

type saturationResult struct {
	Done        bool
	Event       *moira.NotificationEvent
	TriggerData *moira.TriggerData
}

func (worker *FetchEventsWorker) saturate(event *moira.NotificationEvent, triggerData *moira.TriggerData) saturationResult {
	// done == True means the event is ready and can be used
	// done == False means that saturation is not yet complete, this event should be delayed

	logger := logging.GetLogger(event.TriggerID)

	if event.FanTaskID != "" {
		// a request to fan has already been made

		response, err := worker.Fan.CheckProgress(event.FanTaskID)

		// a retry should happen even if CheckProgress returned an error
		// because the error may be temporary
		var shouldRetry bool
		var result saturationResult
		if err != nil {
			// if Fan returned an error, don't use the data it returns
			shouldRetry = true
			result = saturationResult{
				Event:       event,
				TriggerData: triggerData,
			}
			logger.ErrorE("failed to check fan progress", map[string]interface{}{
				"Error":   err.Error(),
				"Event":   event,
				"Trigger": triggerData,
			})
		} else {
			shouldRetry = !response.Done
			// use the data returned by Fan even if it's not done yet
			result = saturationResult{
				Done:        response.Done,
				Event:       response.Event,
				TriggerData: response.TriggerData,
			}
		}

		if shouldRetry {
			// checking for timeouts
			if time.Now().Unix()-event.WaitingForFanSince > FanMaxWait {
				logger.ErrorE("fan task timed out, abandoned", map[string]interface{}{
					"Event":   result.Event,
					"Trigger": result.TriggerData,
				})
				worker.Fan.ApplyFallbacks(result.Event, result.TriggerData, result.TriggerData.Saturation)
				result.Done = true
				return result
			}
			// not timed out yet, retrying
			if err := worker.delayEventAndLog(event, FanDelay); err != nil {
				worker.Fan.ApplyFallbacks(result.Event, result.TriggerData, result.TriggerData.Saturation)
				result.Done = true
				return result
			}
			// successfully scheduled a retry
			return saturationResult{
				Done: false,
			}
		}

		// fan returned OK
		return result

	} else {
		// no request made yet

		request := fan.Request{
			Event:       *event,
			TriggerData: *triggerData,
		}
		taskID, err := worker.Fan.SendRequest(request)
		if event.WaitingForFanSince == 0 {
			event.WaitingForFanSince = time.Now().Unix()
		}
		if err != nil {
			// some error happened when requesting ventilation
			// normally we should retry the request
			// however, if we have been retrying for `FanMaxWait`, then it's time to give up
			// and use the fallbacks
			if event.WaitingForFanSince != 0 {
				alreadyWaitedFor := time.Now().Unix() - event.WaitingForFanSince
				if alreadyWaitedFor > FanMaxWait {
					worker.Fan.ApplyFallbacks(event, triggerData, triggerData.Saturation)
					return saturationResult{
						Done:        true,
						Event:       event,
						TriggerData: triggerData,
					}
				}
			}
		}

		event.FanTaskID = taskID
		// wait for saturation to finish
		if err := worker.delayEventAndLog(event, FanDelay); err != nil {
			worker.Fan.ApplyFallbacks(event, triggerData, triggerData.Saturation)
			return saturationResult{
				Done:        true,
				Event:       event,
				TriggerData: triggerData,
			}
		}
		return saturationResult{
			Done: false,
		}
	}
}

func (worker *FetchEventsWorker) delayEventAndLog(event *moira.NotificationEvent, delay int64) error {
	logger := logging.GetLogger(event.TriggerID)
	logger.InfoE("Delaying event", map[string]interface{}{
		"Event":        event,
		"DelaySeconds": delay,
	})

	if err := worker.Database.AddDelayedNotificationEvent(*event, time.Now().Unix()+delay); err != nil {
		logger.ErrorE("Could not delay event", map[string]interface{}{
			"Event": event,
			"Error": err.Error(),
		})
		return err
	}
	return nil
}

func (worker *FetchEventsWorker) processTriggerAncestors(event *moira.NotificationEvent) ([]moira.NotificationEvent, []string, error) {
	ancestors, err := worker.TriggerInheritanceDatabase.GetAllAncestors(event.TriggerID)
	if err != nil {
		return nil, nil, err
	}

	type triggerMetric struct {
		TriggerID string
		Metric    string
	}
	ancestorsOfEvent := make([]triggerMetric, 0)
	ancestorsForLog := make([]string, 0)

	if event.State == moira.OK {
		parentEvents, err := worker.Database.GetParentEvents(event.TriggerID, event.Metric)
		if err != nil {
			worker.Logger.ErrorF(
				"Could not get parent events for event [%s:%s:%s]: %v. Disabling trigger inheritance!",
				event.TriggerID, event.Metric, event.State, err,
			)
		}
		for parentTriggerID, parentMetrics := range parentEvents {
			for _, parentMetric := range parentMetrics {
				ancestorsOfEvent = append(ancestorsOfEvent, triggerMetric{parentTriggerID, parentMetric})
				ancestorsForLog = append(ancestorsForLog, parentTriggerID+":"+parentMetric)
			}
		}

		for _, ancestor := range ancestorsOfEvent {
			_ = worker.Database.DeleteChildEvents(
				ancestor.TriggerID, ancestor.Metric,
				event.TriggerID, []string{event.Metric},
			)
		}
	} else {
		for _, ancestorChain := range ancestors {
			ancestorID, ancestorMetric, err := worker.processChainOfAncestors(event, ancestorChain)
			switch {
			case err != nil:
				worker.Logger.ErrorF(
					"Could not get ancestors of trigger [%s]: %v. Disabling trigger inheritance!",
					event.TriggerID, err,
				)
				return nil, nil, err
			case ancestorID != "":
				ancestorsOfEvent = append(ancestorsOfEvent, triggerMetric{ancestorID, ancestorMetric})
				ancestorsForLog = append(ancestorsForLog, ancestorID+":"+ancestorMetric)
			}
		}

		for _, ancestor := range ancestorsOfEvent {
			_ = worker.Database.AddChildEvents(
				ancestor.TriggerID, ancestor.Metric,
				event.TriggerID, []string{event.Metric},
			)
		}

		if event.IsForceSent {
			parents, err := worker.Database.GetParentEvents(event.TriggerID, event.Metric)
			if err != nil {
				worker.Logger.ErrorF(
					"Could not get parent events for event [%s:%s:%s]: %v. Forced notifications may be broken",
					event.TriggerID, event.Metric, event.State, err,
				)
			}
			for parentTriggerID, parentMetrics := range parents {
				for _, parentMetric := range parentMetrics {
					_ = worker.Database.DeleteChildEvents(
						parentTriggerID, parentMetric,
						event.TriggerID, []string{event.Metric},
					)
				}
			}
		}
	}
	// ancestorsOfEvent now contains all triggers that have changed state (TODO: rewrite this comment)

	result := make([]moira.NotificationEvent, len(ancestorsOfEvent))
	for i, ancestor := range ancestorsOfEvent {
		newEvent := *event
		newEvent.OverriddenByAncestor = true
		newEvent.AncestorTriggerID = ancestor.TriggerID
		newEvent.AncestorMetric = ancestor.Metric
		result[i] = newEvent
	}
	return result, ancestorsForLog, nil
}

func (worker *FetchEventsWorker) processChainOfAncestors(event *moira.NotificationEvent, ancestorChain []string) (string, string, error) {
	ancestorStates, err := worker.Database.GetTriggerLastChecks(ancestorChain)
	if err != nil {
		return "", "", err
	}

	metricTag := getMetricTag(event.Metric)
	for _, ancestorID := range ancestorChain {
		state := ancestorStates[ancestorID]
		if state == nil {
			continue
		}
		ancestorMetrics := ancestorStates[ancestorID].Metrics
		for ancestorMetric, ancestorMetricState := range ancestorMetrics {
			if isAncestorEvent(ancestorMetricState, event) {
				if len(ancestorMetrics) == 1 {
					// all metrics of the descendant are overridden
					return ancestorID, ancestorMetric, nil
				} else {
					// only matching metrics are overridden
					if metricTag == getMetricTag(ancestorMetric) {
						return ancestorID, ancestorMetric, nil
					}
				}
			}
		}
	}

	// no suitable ancestor found
	return "", "", nil
}

func isAncestorEvent(ancestorMetricState *moira.MetricState, currentEvent *moira.NotificationEvent) bool {
	// if the child event and the ancestor event happened within `timeGap` seconds,
	// Moira decides the child event is caused by the ancestor event
	const timeGap = 5 * 60
	if int64Abs(currentEvent.Timestamp-ancestorMetricState.EventTimestamp) <= timeGap {
		if ancestorMetricState.State == currentEvent.State {
			return true
		}
	}
	return false
}

func getMetricTag(metric string) string {
	return strings.SplitN(metric, ".", 2)[0]
}

func int64Abs(val int64) int64 {
	if val >= 0 {
		return val
	} else {
		return -val
	}
}

func (worker *FetchEventsWorker) getNotificationSubscriptions(event moira.NotificationEvent) (*moira.SubscriptionData, error) {
	if event.SubscriptionID != nil {
		worker.Logger.DebugF("Getting subscriptionID %s for test message", *event.SubscriptionID)
		sub, err := worker.Database.GetSubscription(*event.SubscriptionID)
		if err != nil {
			worker.Metrics.SubsMalformed.Increment()
			return nil, fmt.Errorf("Error while read subscription %s: %s", *event.SubscriptionID, err.Error())
		}
		return &sub, nil
	} else if event.ContactID != "" {
		worker.Logger.DebugF("Getting contactID %s for test message", event.ContactID)
		contact, err := worker.Database.GetContact(event.ContactID)
		if err != nil {
			return nil, fmt.Errorf("Error while read contact %s: %s", event.ContactID, err.Error())
		}
		sub := &moira.SubscriptionData{
			ID:                "testSubscription",
			User:              contact.User,
			ThrottlingEnabled: false,
			Enabled:           true,
			Tags:              make([]string, 0),
			Contacts:          []string{contact.ID},
			Schedule:          moira.ScheduleData{},
		}
		return sub, nil
	}

	return nil, nil
}

func subset(first, second []string) bool {
	set := make(map[string]bool)
	for _, value := range second {
		set[value] = true
	}

	for _, value := range first {
		if !set[value] {
			return false
		}
	}

	return true
}
