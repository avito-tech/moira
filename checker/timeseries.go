package checker

import (
	"fmt"
	"math"

	"go.avito.ru/DO/moira/expression"
	"go.avito.ru/DO/moira/target"
)

type triggerTimeSeries struct {
	Main       []*target.TimeSeries `json:"main,omitempty"`
	Additional []*target.TimeSeries `json:"additional,omitempty"`
}

// ErrWrongTriggerTarget represents inconsistent number of timeseries
type ErrWrongTriggerTarget int

// ErrWrongTriggerTarget implementation for given number of found timeseries
func (err ErrWrongTriggerTarget) Error() string {
	return fmt.Sprintf("Target t%v has more than one timeseries", int(err))
}

func (triggerChecker *TriggerChecker) getTimeSeries(from, until int64) (*triggerTimeSeries, []string, error) {
	triggerTimeSeries := &triggerTimeSeries{
		Main:       make([]*target.TimeSeries, 0),
		Additional: make([]*target.TimeSeries, 0),
	}
	metricsArr := make([]string, 0)

	isSimpleTrigger := triggerChecker.trigger.IsSimple()
	for targetIndex, tar := range triggerChecker.trigger.Targets {
		result, err := target.EvaluateTarget(triggerChecker.Database, tar, from, until, isSimpleTrigger)
		if err != nil {
			return nil, nil, err
		}

		if targetIndex == 0 {
			triggerTimeSeries.Main = result.TimeSeries
		} else {
			timeSeriesCount := len(result.TimeSeries)
			switch {
			case timeSeriesCount == 0:
				if len(result.Metrics) == 0 {
					triggerTimeSeries.Additional = append(triggerTimeSeries.Additional, nil)
				} else {
					return nil, nil, fmt.Errorf("Target t%v has no timeseries", targetIndex+1)
				}
			case timeSeriesCount > 1:
				return nil, nil, ErrWrongTriggerTarget(targetIndex + 1)
			default:
				triggerTimeSeries.Additional = append(triggerTimeSeries.Additional, result.TimeSeries[0])
			}
		}

		metricsArr = append(metricsArr, result.Metrics...)
	}

	triggerChecker.cleanupMetrics(metricsArr, triggerChecker.Until)
	return triggerTimeSeries, metricsArr, nil
}

func (triggerChecker *TriggerChecker) getRemoteTimeSeries(from, until int64) (*triggerTimeSeries, error) {
	triggerTimeSeries := &triggerTimeSeries{
		Main:       make([]*target.TimeSeries, 0),
		Additional: make([]*target.TimeSeries, 0),
	}

	pullURL := triggerChecker.Config.PullURL
	for i, tar := range triggerChecker.trigger.Targets {
		timeseries, err := triggerChecker.PullRemote(pullURL, from, until, []string{tar}) // TODO pull all with one query
		if err != nil {
			return nil, err
		}
		// t1
		if i == 0 {
			triggerTimeSeries.Main = timeseries
		} else {
			switch len(timeseries) {
			case 0:
				triggerTimeSeries.Additional = append(triggerTimeSeries.Additional, nil)
				// TODO check
				//if len(timeseries.Metrics) == 0 {
				//	triggerTimeSeries.Additional = append(triggerTimeSeries.Additional, nil)
				//} else {
				//	return nil, fmt.Errorf("target t%v has no timeseries", i+1)
				//}
			case 1:
				triggerTimeSeries.Additional = append(triggerTimeSeries.Additional, timeseries[0])
			default:
				return nil, ErrWrongTriggerTarget(i + 1)
			}
		}
	}
	return triggerTimeSeries, nil
}

func (*triggerTimeSeries) getMainTargetName() string {
	return "t1"
}

func (*triggerTimeSeries) getAdditionalTargetName(targetIndex int) string {
	return fmt.Sprintf("t%v", targetIndex+2)
}

func (triggerTimeSeries *triggerTimeSeries) getExpressionValues(firstTargetTimeSeries *target.TimeSeries, valueTimestamp int64) (*expression.TriggerExpression, bool) {
	expressionValues := &expression.TriggerExpression{
		AdditionalTargetsValues: make(map[string]float64, len(triggerTimeSeries.Additional)),
	}
	firstTargetValue := firstTargetTimeSeries.GetTimestampValue(valueTimestamp)
	if IsInvalidValue(firstTargetValue) {
		return expressionValues, false
	}
	expressionValues.MainTargetValue = firstTargetValue

	for targetNumber := 0; targetNumber < len(triggerTimeSeries.Additional); targetNumber++ {
		additionalTimeSeries := triggerTimeSeries.Additional[targetNumber]
		if additionalTimeSeries == nil {
			return expressionValues, false
		}
		tnValue := additionalTimeSeries.GetTimestampValue(valueTimestamp)
		if IsInvalidValue(tnValue) {
			return expressionValues, false
		}
		expressionValues.AdditionalTargetsValues[triggerTimeSeries.getAdditionalTargetName(targetNumber)] = tnValue
	}
	return expressionValues, true
}

// IsInvalidValue checks trigger for Inf and NaN. If it is then trigger is not valid
func IsInvalidValue(val float64) bool {
	if math.IsNaN(val) {
		return true
	}
	if math.IsInf(val, 0) {
		return true
	}
	return false
}

// hasOnlyWildcards checks given targetTimeSeries for only wildcards
func (triggerTimeSeries *triggerTimeSeries) hasOnlyWildcards() bool {
	for _, timeSeries := range triggerTimeSeries.Main {
		if !timeSeries.Wildcard {
			return false
		}
	}
	return len(triggerTimeSeries.Main) > 0
}
