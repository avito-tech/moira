package worker

import (
	"fmt"
	"runtime/debug"
	"time"

	"go.avito.ru/DO/moira/checker"
)

func (worker *Checker) perform(triggerIDs []string, cacheTTL time.Duration, isPullType bool) {
	for _, triggerID := range triggerIDs {
		if worker.needHandleTrigger(triggerID, cacheTTL) {
			if isPullType {
				worker.pullTriggersToCheck <- triggerID
			} else {
				worker.triggersToCheck <- triggerID
			}
		}
	}
}

// needHandleTrigger finds out if it is too early to handle the trigger again
func (worker *Checker) needHandleTrigger(triggerID string, cacheTTL time.Duration) bool {
	// first use local cache in order to avoid redundant DB calls
	err := worker.Cache.Add(triggerID, true, cacheTTL)
	if err != nil {
		return false
	}

	// if enough time has passed according to local data, then try to query DB
	locked, _ := worker.Database.SetTriggerCoolDown(triggerID, int(cacheTTL.Seconds()))
	if !locked {
		// delete local lock in case some other checker
		// has recently processed this trigger
		worker.Cache.Delete(triggerID)
	}

	return locked
}

func (worker *Checker) triggerHandler(ch <-chan string) error {
	for {
		triggerID, ok := <-ch
		if !ok {
			return nil
		}
		worker.handle(triggerID)
	}
}

func (worker *Checker) handle(triggerID string) {
	defer func() {
		if r := recover(); r != nil {
			worker.Metrics.HandleError.Increment()
			worker.Logger.TracePanic(fmt.Sprintf("Panic while perform trigger %s", triggerID), map[string]interface{}{
				"value": fmt.Sprintf("%v", r),
				"stack": debug.Stack(),
			})
		}
	}()
	if err := worker.handleTriggerToCheck(triggerID); err != nil {
		worker.Metrics.HandleError.Increment()
		worker.Logger.ErrorF("Failed to perform trigger: %s error: %s", triggerID, err.Error())
	}
}

func (worker *Checker) handleTriggerToCheck(triggerID string) error {
	acquired, err := worker.Database.SetTriggerCheckLock(triggerID)
	if !acquired || err != nil {
		return err
	}

	defer worker.Metrics.CheckTime.UpdateSince(time.Now())
	return worker.checkTrigger(triggerID)
}

func (worker *Checker) checkTrigger(triggerID string) error {
	defer worker.Database.DeleteTriggerCheckLock(triggerID)

	triggerChecker := checker.TriggerChecker{
		TriggerID: triggerID,
		Database:  worker.Database,
		Config:    worker.Config,
		Statsd:    worker.Metrics,
	}

	err := triggerChecker.InitTriggerChecker()
	if err != nil {
		if err == checker.ErrTriggerNotExists {
			return nil
		}
		return err
	}

	return triggerChecker.Check()
}
