package target

import (
	"math"

	"github.com/go-graphite/carbonapi/expr/types"
)

// TimeSeries is abstraction over carbon-api expr.MetricData type
type TimeSeries struct {
	types.MetricData
	Wildcard bool `json:"wildcard"`
}

// GetTimestampValue gets value of given timestamp index, if value is Nil, then return NaN
func (timeSeries *TimeSeries) GetTimestampValue(valueTimestamp int64) float64 {
	if valueTimestamp < int64(timeSeries.StartTime) {
		return math.NaN()
	}
	valueIndex := int((valueTimestamp - int64(timeSeries.StartTime)) / int64(timeSeries.StepTime))
	if len(timeSeries.IsAbsent) > valueIndex && timeSeries.IsAbsent[valueIndex] {
		return math.NaN()
	}
	if len(timeSeries.Values) <= valueIndex {
		return math.NaN()
	}
	return timeSeries.Values[valueIndex]
}
