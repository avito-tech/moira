package worker

import (
	"time"

	"go.avito.ru/DO/moira"
)

func (worker *Checker) metricsChecker(metricEventsChannel <-chan *moira.MetricEvent) error {
	for {
		metricEvent, ok := <-metricEventsChannel
		if !ok {
			close(worker.triggersToCheck)
			worker.Logger.Info("Checking for new events stopped")
			return nil
		}
		if err := worker.handleMetricEvent(metricEvent); err != nil {
			worker.Logger.ErrorF("Failed to handle metricEvent: %s", err.Error())
		}
	}
}

func (worker *Checker) handleMetricEvent(metricEvent *moira.MetricEvent) error {
	worker.lastData = time.Now().UTC().Unix()
	pattern := metricEvent.Pattern
	triggerIds, err := worker.Database.GetPatternTriggerIDs(pattern)
	if err != nil {
		return err
	}
	// Cleanup pattern and its metrics if this pattern doesn't match to any trigger
	if len(triggerIds) == 0 {
		if err := worker.Database.RemovePatternWithMetrics(pattern); err != nil {
			return err
		}
	}
	worker.perform(triggerIds, worker.Config.CheckInterval, false)
	return nil
}
