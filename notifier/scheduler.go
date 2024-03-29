package notifier

import (
	"fmt"
	"math"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
)

// Scheduler implements event scheduling functionality
type Scheduler interface {
	ScheduleNotification(
		next time.Time, throttled bool, event moira.NotificationEvent, trigger moira.TriggerData,
		contact moira.ContactData, sendfail int, needAck bool,
	) *moira.ScheduledNotification

	CalculateBackoff(failCount int) time.Duration

	GetDeliveryInfo(
		now time.Time, event moira.NotificationEvent, throttledOld bool, sendfail int,
	) (time.Time, bool)
}

// StandardScheduler represents standard event scheduling
type StandardScheduler struct {
	logger   *logging.Logger
	database moira.Database
	metrics  *metrics.NotifierMetrics
}

type throttlingLevel struct {
	duration time.Duration
	delay    time.Duration
	count    int64
}

// NewScheduler is initializer for StandardScheduler
func NewScheduler(database moira.Database, metrics *metrics.NotifierMetrics) *StandardScheduler {
	return &StandardScheduler{
		database: database,
		logger:   logging.GetLogger(""),
		metrics:  metrics,
	}
}

func (scheduler *StandardScheduler) CalculateBackoff(failCount int) time.Duration {
	if failCount < 7 {
		// this is always less than 64 minutes
		return time.Duration(math.Pow(2, float64(failCount))) * time.Minute
	} else {
		return 64 * time.Minute
	}
}

func (scheduler *StandardScheduler) GetDeliveryInfo(
	now time.Time, event moira.NotificationEvent,
	throttledOld bool, failCount int,
) (next time.Time, throttled bool) {
	if failCount > 0 {
		next = now.Add(scheduler.CalculateBackoff(failCount))
		throttled = throttledOld
	} else {
		if event.State == moira.TEST {
			next = now
			throttled = false
		} else {
			next, throttled = scheduler.calculateNextDelivery(now, &event)
		}
	}
	return
}

// ScheduleNotification is realization of scheduling event, based on trigger and subscription time intervals and triggers settings
func (scheduler *StandardScheduler) ScheduleNotification(
	next time.Time, throttled bool, event moira.NotificationEvent, trigger moira.TriggerData,
	contact moira.ContactData, sendfail int, needAck bool,
) *moira.ScheduledNotification {
	notification := &moira.ScheduledNotification{
		Event:     event,
		Trigger:   trigger,
		Contact:   contact,
		Throttled: throttled,
		SendFail:  sendfail,
		Timestamp: next.Unix(),
		NeedAck:   needAck,
	}
	scheduler.logger.DebugF(
		"Scheduled notification for contact %s:%s trigger %s at %s (%d)",
		contact.Type, contact.Value, trigger.Name,
		next.Format("2006/01/02 15:04:05"), next.Unix(),
	)

	return notification
}

func (scheduler *StandardScheduler) calculateNextDelivery(now time.Time, event *moira.NotificationEvent) (time.Time, bool) {
	// if trigger switches more than .count times in .duration seconds, delay next delivery for .delay seconds
	// processing stops after first condition matches
	throttlingLevels := []throttlingLevel{
		{3 * time.Hour, time.Hour, 20},
		{time.Hour, time.Hour / 2, 10},
	}

	alarmFatigue := false

	next, beginning := scheduler.database.GetTriggerThrottling(event.TriggerID)

	if next.After(now) {
		alarmFatigue = true
	} else {
		next = now
	}

	subscription, err := scheduler.database.GetSubscription(moira.UseString(event.SubscriptionID))
	if err != nil {
		scheduler.metrics.SubsMalformed.Increment()
		scheduler.logger.DebugF("Failed get subscription by id: %s. %v", moira.UseString(event.SubscriptionID), err)
		return next, alarmFatigue
	}

	if subscription.ThrottlingEnabled {
		if next.After(now) {
			scheduler.logger.DebugF("Using existing throttling for trigger %s: %s", event.TriggerID, next)
		} else {
			for _, level := range throttlingLevels {
				from := now.Add(-level.duration)
				if from.Before(beginning) {
					from = beginning
				}
				count := scheduler.database.GetNotificationEventCount(event.TriggerID, from.Unix())
				if count >= level.count {
					next = now.Add(level.delay)
					scheduler.logger.DebugF("Trigger %s switched %d times in last %s, delaying next notification for %s", event.TriggerID, count, level.duration, level.delay)
					if err = scheduler.database.SetTriggerThrottling(event.TriggerID, next); err != nil {
						scheduler.logger.ErrorF("Failed to set trigger throttling timestamp: %s", err)
					}
					alarmFatigue = true
					break
				} else if count == level.count-1 {
					alarmFatigue = true
				}
			}
		}
	} else {
		next = now
	}
	next, err = calculateNextDelivery(&subscription.Schedule, next)
	if err != nil {
		scheduler.logger.ErrorF("Failed to apply schedule for subscriptionID: %s. %s.", moira.UseString(event.SubscriptionID), err)
	}
	return next, alarmFatigue
}

func calculateNextDelivery(schedule *moira.ScheduleData, nextTime time.Time) (time.Time, error) {

	if len(schedule.Days) != 0 && len(schedule.Days) != 7 {
		return nextTime, fmt.Errorf("Invalid scheduled settings: %d days defined", len(schedule.Days))
	}

	if len(schedule.Days) == 0 {
		return nextTime, nil
	}

	tzOffset := time.Duration(moira.FixedTzOffsetMinutes) * time.Minute
	localNextTime := nextTime.Add(-tzOffset).Truncate(time.Minute)
	beginOffset := time.Duration(schedule.StartOffset) * time.Minute
	endOffset := time.Duration(schedule.EndOffset) * time.Minute
	localNextTimeDay := localNextTime.Truncate(24 * time.Hour)
	localNextWeekday := int(localNextTimeDay.Weekday()+6) % 7

	if schedule.Days[localNextWeekday].Enabled &&
		(localNextTime.Equal(localNextTimeDay.Add(beginOffset)) || localNextTime.After(localNextTimeDay.Add(beginOffset))) &&
		(localNextTime.Equal(localNextTimeDay.Add(endOffset)) || localNextTime.Before(localNextTimeDay.Add(endOffset))) {
		return nextTime, nil
	}

	// find first allowed day
	for i := 0; i < 8; i++ {
		nextLocalDayBegin := localNextTimeDay.Add(time.Duration(i*24) * time.Hour)
		nextLocalWeekDay := int(nextLocalDayBegin.Weekday()+6) % 7
		if localNextTime.After(nextLocalDayBegin.Add(beginOffset)) {
			continue
		}
		if !schedule.Days[nextLocalWeekDay].Enabled {
			continue
		}
		return nextLocalDayBegin.Add(beginOffset + tzOffset), nil
	}

	return nextTime, fmt.Errorf("Can not find allowed schedule day")
}
