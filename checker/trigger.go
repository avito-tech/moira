package checker

import (
	"time"

	"github.com/pkg/errors"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/silencer"
)

// TriggerChecker represents data, used for handling new trigger state
type TriggerChecker struct {
	Config   *Config
	Database moira.Database
	Statsd   *metrics.CheckerMetrics

	logger   moira.Logger
	silencer *silencer.Silencer

	TriggerID    string
	CheckStarted int64
	From, Until  int64

	trigger     *moira.Trigger
	ttl         int64
	ttlState    string
	lastCheck   *moira.CheckData
	maintenance moira.Maintenance

	forced      map[string]bool    // set of metrics' forced notifications
	eventsBatch *moira.EventsBatch // grouper for created moira.NotificationEvent`s
}

// ErrTriggerNotExists used if trigger to check does not exists
var ErrTriggerNotExists = errors.New("trigger does not exists")

// InitTriggerChecker initialize new triggerChecker data
// if trigger does not exists then return ErrTriggerNotExists error
func (triggerChecker *TriggerChecker) InitTriggerChecker() error {
	now := time.Now().Unix()

	// these if clauses are strictly for tests
	// normally initialized TriggerChecker
	// has these values been equal to zero
	if triggerChecker.CheckStarted == 0 {
		triggerChecker.CheckStarted = now
	}
	if triggerChecker.Until == 0 {
		triggerChecker.Until = now
	}

	trigger, err := triggerChecker.Database.GetTrigger(triggerChecker.TriggerID)
	if err != nil {
		if err == database.ErrNil {
			return ErrTriggerNotExists
		}
		return err
	}
	triggerChecker.trigger = trigger
	triggerChecker.ttl = trigger.TTL

	if trigger.TTLState != nil {
		triggerChecker.ttlState = *trigger.TTLState
	} else {
		triggerChecker.ttlState = moira.NODATA
	}

	// start of time interval for trigger checking is usually now() - ttl
	// but if ttl == 0, then default value is used
	ttlGap := triggerChecker.ttl
	if ttlGap == 0 {
		ttlGap = 600
	}
	if trigger.PendingInterval > 0 {
		ttlGap += trigger.PendingInterval
	}
	triggerChecker.From = triggerChecker.Until - ttlGap

	lastCheck, err := triggerChecker.Database.GetOrCreateTriggerLastCheck(triggerChecker.TriggerID)
	if err != nil {
		return err
	}
	triggerChecker.lastCheck = lastCheck

	if lastCheck.Timestamp == 0 {
		// if there were no previous checks of this trigger then null value will be created
		// set the timestamp of such a value as start of time interval
		lastCheck.Timestamp = triggerChecker.From
	} else if lastCheck.Timestamp < triggerChecker.From {
		// on the other hand, if the last check has finished before default interval's start
		// then start this interval from the last check
		triggerChecker.From = lastCheck.Timestamp
	}
	// all in all, triggerChecker.From <= lastCheck.Timestamp and triggerChecker.From <= now() - ttlGap

	triggerChecker.logger = logging.GetLogger(triggerChecker.TriggerID)
	triggerChecker.silencer = silencer.NewSilencer(triggerChecker.Database, nil)

	forced, err := triggerChecker.Database.GetTriggerForcedNotifications(triggerChecker.TriggerID)
	if err != nil {
		triggerChecker.forced = make(map[string]bool)
		triggerChecker.logger.ErrorE(
			"Could not get forced notification for trigger",
			map[string]interface{}{
				"TriggerID": triggerChecker.TriggerID,
				"Error":     err.Error(),
			},
		)
	} else {
		triggerChecker.forced = forced
	}

	if triggerChecker.eventsBatch == nil {
		triggerChecker.eventsBatch = moira.NewEventsBatch(triggerChecker.CheckStarted)
	}

	if lastCheck.Version > 0 {
		triggerChecker.maintenance, err = triggerChecker.Database.GetOrCreateMaintenanceTrigger(triggerChecker.TriggerID)
		if err != nil {
			return err
		}
	} else {
		triggerChecker.maintenance = moira.NewMaintenanceFromCheckData(lastCheck)
	}

	return nil
}
