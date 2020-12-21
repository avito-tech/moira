package checker

import (
	"fmt"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/target"
)

const (
	chunkSize           = 20
	checkPointGap int64 = 120
)

// ErrTriggerHasNoTimeSeries used if trigger has no metrics
type ErrTriggerHasNoTimeSeries struct{}

// ErrTriggerHasNoTimeSeries implementation with constant error message
func (err ErrTriggerHasNoTimeSeries) Error() string {
	return fmt.Sprintf("Trigger has no metrics, check your target")
}

// ErrTriggerHasOnlyWildcards used if trigger has only wildcard metrics
type ErrTriggerHasOnlyWildcards struct{}

// ErrTriggerHasOnlyWildcards implementation with constant error message
func (err ErrTriggerHasOnlyWildcards) Error() string {
	return fmt.Sprintf("Trigger never received metrics")
}

// ErrTriggerHasSameTimeSeriesNames used if trigger has two timeseries with same name
type ErrTriggerHasSameTimeSeriesNames struct{}

// ErrTriggerHasSameTimeSeriesNames implementation with constant error message
func (err ErrTriggerHasSameTimeSeriesNames) Error() string {
	return fmt.Sprintf("Trigger has same timeseries names")
}

// Check handle trigger and last check and write new state of trigger, if state were change then write new NotificationEvent
func (triggerChecker *TriggerChecker) Check() error {
	triggerChecker.logger.DebugF("Checking trigger %s", triggerChecker.TriggerID)

	// don't handle trigger at all if it is disabled by maintenance/schedule/etc.
	if triggerChecker.isHandleMetricDisabled(triggerChecker.CheckStarted, moira.WildcardMetric, "Basic check") {
		return nil
	}

	// log call quantity stats
	defer triggerChecker.logger.TraceSelfStats(triggerChecker.TriggerID, time.Now())

	checkData, err := triggerChecker.handleTrigger()
	if err != nil {
		triggerChecker.logger.ErrorE(fmt.Sprintf("Error while handleTrigger: %v", err), map[string]interface{}{
			"trigger_id": triggerChecker.TriggerID,
		})
		checkData, err = triggerChecker.handleErrorCheck(checkData, err)
		if err != nil {
			triggerChecker.logger.ErrorE(fmt.Sprintf("Error while handleErrorCheck: %v", err), map[string]interface{}{
				"trigger_id": triggerChecker.TriggerID,
			})
			return err
		}
	}

	checkData.UpdateScore()
	err = triggerChecker.Database.SetTriggerLastCheck(triggerChecker.TriggerID, &checkData)

	return err
}

func (triggerChecker *TriggerChecker) handleTrigger() (moira.CheckData, error) {
	var (
		checkingError, err error
		triggerTimeSeries  *triggerTimeSeries
	)

	triggerChecker.cleanupMaintenanceMetrics()
	checkData := moira.CheckData{
		State:             moira.OK,
		Timestamp:         triggerChecker.Until,
		EventTimestamp:    triggerChecker.lastCheck.EventTimestamp,
		Score:             triggerChecker.lastCheck.Score,
		Maintenance:       triggerChecker.lastCheck.Maintenance,
		MaintenanceMetric: triggerChecker.lastCheck.MaintenanceMetric,
		Metrics:           triggerChecker.lastCheck.Metrics,
		Version:           triggerChecker.lastCheck.Version,
	}

	// getting time series: either from graphite or from DB
	if triggerChecker.trigger.IsPullType {
		triggerTimeSeries, err = triggerChecker.getRemoteTimeSeries(triggerChecker.From, triggerChecker.Until)
		if err != nil {
			return checkData, err
		}
	} else {
		triggerTimeSeries, _, err = triggerChecker.getTimeSeries(triggerChecker.From, triggerChecker.Until)
		if err != nil {
			return checkData, err
		}
	}

	hasOnlyWildcards := triggerTimeSeries.hasOnlyWildcards()
	triggerChecker.traceHandledData(triggerTimeSeries, hasOnlyWildcards)
	if hasOnlyWildcards {
		return checkData, ErrTriggerHasOnlyWildcards{}
	}

	deleteForcedNotifications := make([]string, 0, len(triggerTimeSeries.Main))
	timeSeriesNamesMap := make(map[string]bool, len(triggerTimeSeries.Main))
	patternMetricsRemoved := false
	if len(triggerTimeSeries.Main) > 0 {
		for _, timeSeries := range triggerTimeSeries.Main {
			triggerChecker.logger.DebugF("[TriggerID:%s] Checking timeSeries %s: %v", triggerChecker.TriggerID, timeSeries.Name, timeSeries.Values)
			triggerChecker.logger.DebugF("[TriggerID:%s][TimeSeries:%s] Checking interval: %v - %v (%vs), step: %v", triggerChecker.TriggerID, timeSeries.Name, timeSeries.StartTime, timeSeries.StopTime, timeSeries.StepTime, timeSeries.StopTime-timeSeries.StartTime)

			if triggerChecker.isHandleMetricDisabled(triggerChecker.CheckStarted, timeSeries.Name, "handleTrigger:checkMainMetrics") {
				continue
			}

			if _, ok := timeSeriesNamesMap[timeSeries.Name]; ok {
				triggerChecker.logger.InfoF("[TriggerID:%s][TimeSeries:%s] Trigger has same time series names", triggerChecker.TriggerID, timeSeries.Name)
				checkingError = ErrTriggerHasSameTimeSeriesNames{}
				continue
			}

			timeSeriesNamesMap[timeSeries.Name] = true
			metricState, deleteMetric, deleteForced, err := triggerChecker.checkTimeSeries(timeSeries, triggerTimeSeries)

			if deleteMetric {
				triggerChecker.logger.InfoF("[TriggerID:%s] Remove metric: '%s'", triggerChecker.TriggerID, timeSeries.Name)
				delete(checkData.Metrics, timeSeries.Name)

				if !patternMetricsRemoved {
					err = triggerChecker.Database.RemovePatternsMetrics(triggerChecker.trigger.Patterns)
					patternMetricsRemoved = true
				}
			} else {
				checkData.Metrics[timeSeries.Name] = metricState
			}

			if deleteForced {
				deleteForcedNotifications = append(deleteForcedNotifications, timeSeries.Name)
			}

			if err != nil {
				return checkData, err
			}
		}
	} else {
		checkingError = ErrTriggerHasNoTimeSeries{}
	}

	// there might be metrics which were present in lastCheck (after prior checks) but are not present at the moment
	// if there are then there is a special action for them
	metricsReplacement := make(map[string]*moira.MetricState, len(checkData.Metrics))
	metricsToDelete := make([]string, 0, len(checkData.Metrics))
	for metricName, metricState := range checkData.Metrics {
		if _, ok := timeSeriesNamesMap[metricName]; ok {
			continue
		}

		// forced notification should be deleted since metricName is not present in triggerTimeSeries.Main
		isForced := triggerChecker.forced[metricName]
		// find out if this metric is to be deleted according to trigger ttlState setting
		noDataState, deleteMetric := triggerChecker.checkForNoData(metricState)
		// find out if its state differs from nodata
		stateDiffers := noDataState != nil && (!metricState.IsNoData || metricState.State != noDataState.State)

		triggerChecker.logger.InfoE(
			fmt.Sprintf("[TriggerID:%s] Processing obsolete metric %s", triggerChecker.TriggerID, metricName),
			map[string]interface{}{
				"no_data_state": noDataState,
				"metric_state":  metricState,
				"state_differs": stateDiffers,
				"metric_name":   metricName,
				"is_forced":     isForced,
				"to_delete":     deleteMetric,
			},
		)

		if isForced {
			deleteForcedNotifications = append(deleteForcedNotifications, metricName)
		}

		if deleteMetric {
			metricsToDelete = append(metricsToDelete, metricName)
			continue
		}

		// `noDataState != nil condition` is here so that IDE could be happy
		if noDataState != nil && stateDiffers {
			if triggerChecker.isHandleMetricDisabled(triggerChecker.CheckStarted, metricName, "handleTrigger:checkMissingMetrics") {
				continue
			}

			metricNewState, err := triggerChecker.compareStates(metricName, noDataState, metricState, isForced)
			if err == nil {
				metricsReplacement[metricName] = metricNewState
			}
		}
	}

	// and now apply this special action if there are some metrics lost
	for metricName, metricState := range metricsReplacement {
		checkData.Metrics[metricName] = metricState
	}
	for _, metricName := range metricsToDelete {
		delete(checkData.Metrics, metricName)
	}

	// deleting forced notifications which have been sent
	for len(deleteForcedNotifications) > 0 {
		// cutting these into chunks so that Redis doesn't choke
		var chunk, rest []string = deleteForcedNotifications, nil
		if len(chunk) > chunkSize {
			chunk, rest = chunk[:chunkSize], chunk[chunkSize:]
		}
		if err := triggerChecker.Database.DeleteTriggerForcedNotifications(triggerChecker.TriggerID, chunk); err != nil {
			triggerChecker.logger.ErrorE("Could not delete forced notification for trigger", map[string]interface{}{
				"TriggerID": triggerChecker.TriggerID,
				"Error":     err.Error(),
			})
		}
		deleteForcedNotifications = rest
	}

	return checkData, checkingError
}

func (triggerChecker *TriggerChecker) handleErrorCheck(checkData moira.CheckData, checkingError error) (moira.CheckData, error) {
	switch checkingError.(type) {
	case ErrTriggerHasNoTimeSeries:
		triggerChecker.logger.DebugF("Trigger %s: %s", triggerChecker.TriggerID, checkingError.Error())
		checkData.State = triggerChecker.ttlState
		checkData.Message = checkingError.Error()
		if triggerChecker.ttl == 0 || triggerChecker.ttlState == moira.DEL {
			return checkData, nil
		}
	case ErrTriggerHasOnlyWildcards:
		triggerChecker.logger.DebugF("Trigger %s: %s", triggerChecker.TriggerID, checkingError.Error())
		if len(checkData.Metrics) == 0 && triggerChecker.ttlState != moira.OK && triggerChecker.ttlState != moira.DEL {
			checkData.State = moira.NODATA
			checkData.Message = checkingError.Error()
			if triggerChecker.ttl == 0 || triggerChecker.ttlState == moira.DEL {
				return checkData, nil
			}
		}
	case target.ErrUnknownFunction:
		triggerChecker.logger.WarnF("Trigger %s: %s", triggerChecker.TriggerID, checkingError.Error())
		checkData.State = moira.EXCEPTION
		checkData.Message = checkingError.Error()
	case ErrWrongTriggerTarget, ErrTriggerHasSameTimeSeriesNames:
		checkData.State = moira.EXCEPTION
		checkData.Message = checkingError.Error()
	default:
		triggerChecker.Statsd.CheckError.Increment()
		triggerChecker.logger.ErrorF("Trigger %s check failed: %s", triggerChecker.TriggerID, checkingError.Error())
		checkData.State = moira.EXCEPTION
		checkData.Message = checkingError.Error()
	}

	return triggerChecker.compareChecks(checkData, triggerChecker.forced[moira.WildcardMetric])
}

func (triggerChecker *TriggerChecker) checkTimeSeries(
	timeSeries *target.TimeSeries, triggerTimeSeries *triggerTimeSeries,
) (lastState *moira.MetricState, deleteMetric bool, deleteForced bool, err error) {
	emptyTimestampValue := int64(timeSeries.StartTime) - moira.MaxI64(triggerChecker.ttl, 3600)
	lastState = triggerChecker.lastCheck.GetOrCreateMetricState(timeSeries.Name, emptyTimestampValue)

	metricStates, err := triggerChecker.getTimeSeriesStepsStates(triggerTimeSeries, timeSeries, lastState)
	if err != nil {
		triggerChecker.logger.ErrorF(
			"getTimeSeriesStepsStates for trigger_id %s caused an error %v, time_series = %s",
			triggerChecker.TriggerID, err, timeSeries.Name,
		)
		return
	}
	triggerChecker.traceMetricStates(metricStates, lastState, timeSeries)

	needForceSend := triggerChecker.forced[timeSeries.Name]
	deleteForced = needForceSend

	for _, currentState := range metricStates {
		triggerChecker.logger.InfoE("About to launch compareStates", map[string]interface{}{
			"trigger_id":    triggerChecker.TriggerID,
			"time_series":   timeSeries.Name,
			"current_state": currentState,
			"last_state":    lastState,
		})
		lastState, err = triggerChecker.compareStates(timeSeries.Name, currentState, lastState, needForceSend)
		if err != nil {
			return
		}
		if lastState.IsForced {
			needForceSend = false
		}
	}

	noDataState, deleteMetric := triggerChecker.checkForNoData(lastState)
	if deleteMetric {
		return
	}

	if noDataState != nil {
		triggerChecker.logger.InfoE("Also compare with noDataState", map[string]interface{}{
			"trigger_id":  triggerChecker.TriggerID,
			"time_series": timeSeries.Name,
			"noDataState": *noDataState,
			"lastState":   lastState,
		})
		lastState, err = triggerChecker.compareStates(timeSeries.Name, noDataState, lastState, needForceSend)
	}

	return
}

func (triggerChecker *TriggerChecker) checkForNoData(lastState *moira.MetricState) (noDataState *moira.MetricState, deleteMetric bool) {
	if triggerChecker.ttl == 0 {
		return nil, false
	}

	if lastState.Timestamp+triggerChecker.ttl >= triggerChecker.lastCheck.Timestamp {
		return nil, false
	}

	if triggerChecker.ttlState == moira.DEL && (lastState.EventTimestamp != 0 || lastState.IsNoData) {
		return nil, true
	}

	noDataState = &moira.MetricState{
		State:      triggerChecker.getNullState(),
		Timestamp:  triggerChecker.CheckStarted,
		Value:      nil,
		Suppressed: lastState.Suppressed,
		IsNoData:   true,
	}
	return noDataState, false
}

// cleanupMaintenanceMetrics leaves only those of maintenance metrics which haven't expired yet
func (triggerChecker *TriggerChecker) cleanupMaintenanceMetrics() {
	maintenanceMetrics := make(map[string]int64, len(triggerChecker.lastCheck.MaintenanceMetric))
	now := time.Now().Unix()
	for k, v := range triggerChecker.lastCheck.MaintenanceMetric {
		if v > now {
			maintenanceMetrics[k] = v
		}
	}
	triggerChecker.lastCheck.MaintenanceMetric = maintenanceMetrics
}

// cleanupMetrics leaves only those metrics which haven't expired yet
func (triggerChecker *TriggerChecker) cleanupMetrics(metrics []string, until int64) {
	if len(metrics) > 0 {
		if err := triggerChecker.Database.RemoveMetricsValues(metrics, until-triggerChecker.Config.MetricsTTLSeconds); err != nil {
			triggerChecker.logger.ErrorF("Failed to remove metric values: metrics = %v, err = %v", metrics, err)
		}
	}
}

// getNullState returns state which is suitable to create null moira.MetricState (if it doesn't exist)
func (triggerChecker *TriggerChecker) getNullState() string {
	if triggerChecker.ttlState == moira.DEL {
		return moira.NODATA
	}
	return triggerChecker.ttlState
}

func (triggerChecker *TriggerChecker) getTimeSeriesState(triggerTimeSeries *triggerTimeSeries, timeSeries *target.TimeSeries, lastState *moira.MetricState, valueTimestamp, checkPoint int64) (*moira.MetricState, error) {
	if valueTimestamp <= checkPoint {
		return nil, nil
	}
	triggerExpression, noEmptyValues := triggerTimeSeries.getExpressionValues(timeSeries, valueTimestamp)
	if !noEmptyValues {
		return nil, nil
	}
	triggerChecker.logger.DebugF(
		"[TriggerID:%s][TimeSeries:%s] Values for ts %v: MainTargetValue: %v, additionalTargetValues: %v",
		triggerChecker.TriggerID, timeSeries.Name, valueTimestamp,
		triggerExpression.MainTargetValue, triggerExpression.AdditionalTargetsValues,
	)

	triggerExpression.WarnValue = triggerChecker.trigger.WarnValue
	triggerExpression.ErrorValue = triggerChecker.trigger.ErrorValue
	triggerExpression.PreviousState = lastState.State
	triggerExpression.Expression = triggerChecker.trigger.Expression

	expressionState, err := triggerExpression.Evaluate()
	if err != nil {
		return nil, err
	}

	return &moira.MetricState{
		State:      expressionState,
		Timestamp:  valueTimestamp,
		Value:      &triggerExpression.MainTargetValue,
		Suppressed: lastState.Suppressed,
	}, nil
}

func (triggerChecker *TriggerChecker) getTimeSeriesStepsStates(
	triggerTimeSeries *triggerTimeSeries,
	timeSeries *target.TimeSeries,
	metricLastState *moira.MetricState,
) ([]*moira.MetricState, error) {
	startTime := int64(timeSeries.StartTime)
	stepTime := int64(timeSeries.StepTime)

	checkPoint := metricLastState.GetCheckPoint(checkPointGap)
	triggerChecker.logger.DebugF("[TriggerID:%s][TimeSeries:%s] Checkpoint: %v", triggerChecker.TriggerID, timeSeries.Name, checkPoint)

	metricStates := make([]*moira.MetricState, 0)
	for valueTimestamp := startTime; valueTimestamp < triggerChecker.Until+stepTime; valueTimestamp += stepTime {
		metricNewState, err := triggerChecker.getTimeSeriesState(triggerTimeSeries, timeSeries, metricLastState, valueTimestamp, checkPoint)
		if err != nil {
			return nil, err
		}
		if metricNewState == nil {
			continue
		}

		metricLastState = metricNewState
		metricStates = append(metricStates, metricNewState)
	}
	return metricStates, nil
}

// traceHandledData is separate function because it it too much to trace :)
func (triggerChecker *TriggerChecker) traceHandledData(triggerTimeSeries *triggerTimeSeries, hasOnlyWildcards bool) {
	forcedTotal := len(triggerChecker.forced)
	forced := make(map[string]bool, forcedTotal)

	for key, val := range triggerChecker.forced {
		if val {
			forced[key] = val
		}
	}

	triggerChecker.logger.InfoE(
		fmt.Sprintf("handleTrigger id %s", triggerChecker.TriggerID),
		map[string]interface{}{
			"trigger_id":       triggerChecker.TriggerID,
			"forced_total":     forcedTotal,
			"forced_true":      forced,
			"is_pull_type":     triggerChecker.trigger.IsPullType,
			"from":             triggerChecker.From,
			"until":            triggerChecker.Until,
			"only_wildcards":   hasOnlyWildcards,
			"series_qty":       len(triggerTimeSeries.Main),
			"ttl":              triggerChecker.ttl,
			"ttl_state":        triggerChecker.ttlState,
			"last_check_ts":    triggerChecker.lastCheck.Timestamp,
			"last_metrics_qty": len(triggerChecker.lastCheck.Metrics),
		},
	)
}

// traceMetricStates implements chunked metric states logging (it can be too many of them)
func (triggerChecker *TriggerChecker) traceMetricStates(
	currentStates []*moira.MetricState,
	lastState *moira.MetricState,
	timeSeries *target.TimeSeries,
) {
	statesTotal := len(currentStates)
	timeSeriesName := timeSeries.Name

	triggerChecker.logger.InfoE(
		fmt.Sprintf("Started checkTimeSeries for '%s'", timeSeriesName),
		map[string]interface{}{
			"trigger_id": triggerChecker.TriggerID,
			"ts_name":    timeSeriesName,
			"ts_start":   timeSeries.StartTime,
			"ts_stop":    timeSeries.StopTime,
			"last_state": lastState,
			"states_qty": statesTotal,
			"states":     currentStates,
		},
	)
}
