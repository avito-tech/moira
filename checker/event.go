package checker

import (
	"fmt"
	"time"

	"go.avito.ru/DO/moira"
)

var (
	badStateReminder = map[string]int64{
		moira.ERROR:  86400,
		moira.NODATA: 86400,
	}

	childMetricMessage = "This metric has not returned to OK state!"
)

const maintenanceTimeFormat = "2006-01-02 15:04:05.999"

// compareChecks runs once per TriggerChecker.Check call
func (triggerChecker *TriggerChecker) compareChecks(
	currCheck moira.CheckData,
	needForceSend bool,
) (moira.CheckData, error) {

	var (
		lastEventTs int64
		lastCheck   = triggerChecker.lastCheck
		triggerID   = triggerChecker.TriggerID

		// timestamp of the current trigger state is timestamp of new event by default
		// (assuming there will be a new event)
		currEventTs = currCheck.Timestamp
	)

	// transiting timestamp of last event happened to the current state
	if lastCheck.EventTimestamp != 0 {
		currCheck.EventTimestamp = lastCheck.EventTimestamp
		lastEventTs = lastCheck.EventTimestamp
	} else {
		currCheck.EventTimestamp = currEventTs
		lastEventTs = lastCheck.Timestamp
	}
	currCheck.Suppressed = false

	needSend, keepPending, message := needSendEvent(
		currCheck.State,
		lastCheck.State,
		currEventTs,
		lastEventTs,
		lastCheck.IsPending,
		triggerChecker.trigger.PendingInterval,
		lastCheck.Suppressed,
	)
	if !keepPending {
		currCheck.IsPending = false
	}

	if needForceSend {
		if currCheck.State == lastCheck.State && currCheck.State != moira.OK && currCheck.State != moira.NODATA {
			currEventTs = lastEventTs // change (default) value of the current event
			needSend = true
			message = &childMetricMessage

			// if there is forced notification for the whole trigger's state, it overrides all existing ones for metrics
			err := triggerChecker.Database.DeleteTriggerForcedNotification(triggerID, moira.WildcardMetric)
			if err != nil {
				triggerChecker.logger.ErrorE("Could not delete forced notification for trigger", map[string]interface{}{
					"TriggerID": triggerID,
					"Error":     err.Error(),
				})
			}
		}
	}

	if !needSend {
		if keepPending {
			// keepPending means that the state of trigger has changed
			// and there is event to send, but time needs to pass
			//
			// event should be emitted only if the old state doesn't return
			// so it is passed to the current state in order to continue
			// keeping "state has changed" flag
			currCheck.State = lastCheck.State
			currCheck.IsPending = true

			// also set the timestamp of the current event if it is the first state's changing
			if !lastCheck.IsPending {
				currCheck.EventTimestamp = currEventTs
			}
		}

		return currCheck, nil
	}

	// pushing the new event and memorizing its timestamp
	currCheck.EventTimestamp = currEventTs
	event := &moira.NotificationEvent{
		IsForceSent:    needForceSend,
		IsTriggerEvent: true,
		TriggerID:      triggerID,
		State:          currCheck.State,
		OldState:       lastCheck.State,
		Timestamp:      currEventTs,
		Metric:         triggerChecker.trigger.Name,
		Message:        message,
		Batch:          triggerChecker.eventsBatch,
		HasSaturations: len(triggerChecker.trigger.Saturation) != 0,
	}
	if event.Message == nil {
		event.Message = &currCheck.Message
	}
	err := triggerChecker.Database.PushNotificationEvent(event)
	triggerChecker.logger.InfoE(fmt.Sprintf("Pushed notification event (check), err = %v", err), event)

	return currCheck, err
}

func (triggerChecker *TriggerChecker) compareStates(
	metric string,
	currState *moira.MetricState,
	lastState *moira.MetricState,
	needForceSend bool,
) (*moira.MetricState, error) {

	var (
		lastEventTs int64
		triggerID   = triggerChecker.TriggerID

		// timestamp of the current metric state is timestamp of new event by default
		// (assuming there will be a new event)
		currEventTs = currState.Timestamp
	)

	// transiting timestamp of last event happened to the current state
	if lastState.EventTimestamp != 0 {
		currState.EventTimestamp = lastState.EventTimestamp
		lastEventTs = lastState.EventTimestamp
	} else {
		currState.EventTimestamp = currEventTs
		lastEventTs = lastState.Timestamp
	}
	currState.Suppressed = false

	needSend, keepPending, message := needSendEvent(
		currState.State,
		lastState.State,
		currEventTs,
		lastEventTs,
		lastState.IsPending,
		triggerChecker.trigger.PendingInterval,
		lastState.Suppressed,
	)
	if !keepPending {
		currState.IsPending = false
	}

	if needForceSend {
		if currState.State == lastState.State && currState.State != moira.OK && currState.State != moira.NODATA {
			// if there is forced notification, but the state hasn't changed, then set IsForced flag on
			currState.IsForced = true
			// also change (default) value of the current event
			currEventTs = lastEventTs

			needSend = true
			message = &childMetricMessage
		}
	}

	if !needSend {
		if keepPending {
			// keepPending means that the state of a metric has changed
			// and there is event to send, but time needs to pass
			//
			// event should be emitted only if the old state doesn't return
			// so it is passed to the current state in order to continue
			// keeping "state has changed" flag
			currState.State = lastState.State
			currState.IsPending = true

			// also set the timestamp of the current event if it is the first state's changing
			if !lastState.IsPending {
				currState.EventTimestamp = currEventTs
			}

			// timestamp of pending NODATA state must be transited
			// in case metric continues to be out of data points
			// because new NODATA state will have Timestamp equaling
			// timestamp of this check, and it might not be enough
			// to cover be pending interval
			if currState.IsNoData {
				currState.Timestamp = lastState.Timestamp
			}
		}

		return currState, nil
	}

	// when the trigger is handled in the very beginning of its schedule, its metrics states sequence may contain states
	// which are out of the schedule, so double check it here
	if triggerChecker.isHandleMetricDisabled(currState.Timestamp, metric, "compareStates:beforePushEvent") {
		currState.Suppressed = true
		return currState, nil
	}

	// pushing the new event and memorizing its timestamp
	currState.EventTimestamp = currEventTs
	event := &moira.NotificationEvent{
		IsForceSent:    needForceSend,
		TriggerID:      triggerID,
		State:          currState.State,
		OldState:       lastState.State,
		Value:          currState.Value,
		OldValue:       lastState.Value,
		Timestamp:      currEventTs,
		Metric:         metric,
		Message:        message,
		Batch:          triggerChecker.eventsBatch,
		HasSaturations: len(triggerChecker.trigger.Saturation) != 0,
	}
	err := triggerChecker.Database.PushNotificationEvent(event)
	triggerChecker.logger.InfoE(fmt.Sprintf("Pushed notification event (state), err = %v", err), event)

	// if current state has changed to OK then force assign this state to its child triggers
	if currState.State == moira.OK {
		triggerChildren, err := triggerChecker.Database.GetChildEvents(triggerID, metric)
		if err != nil {
			triggerChecker.logger.ErrorE(
				"could not get child events for trigger",
				map[string]interface{}{
					"TriggerID": triggerID,
					"Metric":    metric,
					"Error":     err.Error(),
				},
			)
			return currState, err
		}

		for childTriggerID, childMetrics := range triggerChildren {
			var (
				forcedNotificationDelay int64 = 2 * 60 // seconds
				forcedNotificationTime        = time.Now().Unix() + forcedNotificationDelay
			)

			triggerChecker.logger.InfoE(
				"will force notifications for child triggers",
				map[string]interface{}{
					"ParentTriggerID":       triggerID,
					"ChildTriggerID":        childTriggerID,
					"ChildMetrics":          childMetrics,
					"Delay":                 forcedNotificationDelay,
					"NotificationPlannedAt": forcedNotificationTime,
				},
			)

			err := triggerChecker.Database.AddTriggerForcedNotification(childTriggerID, childMetrics, forcedNotificationTime)
			if err != nil {
				triggerChecker.logger.ErrorE(
					"could not add forced notifications for trigger",
					map[string]interface{}{
						"ParentTriggerID": triggerID,
						"ChildTriggerID":  childTriggerID,
						"ChildMetrics":    childMetrics,
					},
				)
			}
		}
	}

	return currState, err
}

// isHandleMetricDisabled finds out if it is disabled to handle the metric (or the trigger) given
// ts is what is considered to be current timestamp
// metric is the name of the metric (or moira.WildcardMetric if the trigger is processed)
// returns disabled flag; also logs code and reason if handling is disabled indeed
func (triggerChecker *TriggerChecker) isHandleMetricDisabled(ts int64, metric, dataExtra string) (disabled bool) {
	var (
		code   int
		reason string
	)

	defer func() {
		if disabled {
			triggerChecker.logger.InfoE(
				fmt.Sprintf("Handling of metric '%s' (trigger %s) is disabled", metric, triggerChecker.TriggerID),
				map[string]interface{}{
					"id":     triggerChecker.TriggerID,
					"code":   code,
					"reason": reason,
					"extra":  dataExtra,
					"metric": metric,
				},
			)
		}
	}()

	// check trigger schedule
	if !triggerChecker.trigger.Schedule.IsScheduleAllows(ts) {
		code = moira.EventMutedSchedule
		reason = "Handling is disabled due to trigger schedule"
		return true
	}

	// check if metric or any tags are silenced
	isWildcard := metric == moira.WildcardMetric
	if triggerChecker.silencer != nil {
		if !isWildcard && triggerChecker.silencer.IsMetricSilenced(metric, ts) {
			code = moira.EventMutedSilent
			reason = fmt.Sprintf("Handling is disabled due to metric %s being silent", metric)
			return true
		}

		if triggerChecker.silencer.IsTagsSilenced(triggerChecker.trigger.Tags, ts) {
			code = moira.EventMutedSilent
			reason = "Handling is disabled due to tags being silent"
			return true
		}
	}

	// check maintenance for this metric and for the whole trigger as well
	names := []string{metric}
	if !isWildcard {
		names = append(names, moira.WildcardMetric)
	}

	for _, name := range names {
		if maintained, until := triggerChecker.maintenance.Get(name, ts); maintained {
			untilStr := time.Unix(until, 0).Format(maintenanceTimeFormat)
			code = moira.EventMutedMaintenance
			reason = fmt.Sprintf("Handling is disabled due to metric %s maintenance until %s", name, untilStr)
			return true
		}
	}

	return false
}

// needSendEvent can tell whether event should be sent right now, put on hold (pending) or neither
// it can't be both
func needSendEvent(
	currentStateValue string, lastStateValue string,
	currentStateTimestamp int64, lastStateEventTimestamp int64,
	isLastStatePending bool, pendingInterval int64,
	isLastStateSuppressed bool,
) (needSend bool, keepPending bool, message *string) {
	// how much time passed since the last event
	diff := currentStateTimestamp - lastStateEventTimestamp

	// states differ -- probably need to send
	if currentStateValue != lastStateValue {
		// if trigger has pending interval then
		// the first event is put on hold
		// and all others, until interval expires
		// but the first one is always get pending
		keepPending = pendingInterval > 0 && (!isLastStatePending || (diff < pendingInterval))
		needSend = !keepPending
		// it's either send event or put it pending
		return needSend, keepPending, nil
	}

	// states coincide, but they are bad states (moira.NODATA or moira.ERROR)
	// and it's been a long time since the last event
	remindInterval, ok := badStateReminder[currentStateValue]
	if ok && diff >= remindInterval {
		message := fmt.Sprintf("This metric has been in bad state for more than %v hours - please, fix.", remindInterval/3600)
		needSend = true
		return needSend, false, &message
	}

	// other scenarios: states coincide and they are either both good
	// or it hasn't passed enough time
	// the only reason to send is that the last state
	// was muted (maintenance, silent patterns etc.) and it was bad
	needSend = isLastStateSuppressed && lastStateValue != moira.OK
	return needSend, false, nil
}
