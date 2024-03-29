package checker

import (
	"fmt"
	"math"
	"testing"

	pb "github.com/go-graphite/carbonapi/carbonzipperpb3"
	et "github.com/go-graphite/carbonapi/expr/types"
	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/expression"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/target"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestIsInvalidValue(t *testing.T) {
	Convey("values +Inf -Inf and NaN is invalid", t, func() {
		So(IsInvalidValue(math.NaN()), ShouldBeTrue)
		So(IsInvalidValue(math.Inf(-1)), ShouldBeTrue)
		So(IsInvalidValue(math.Inf(1)), ShouldBeTrue)
		So(IsInvalidValue(3.14), ShouldBeFalse)
	})
}

func TestGetTimeSeries(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	pattern := "super.puper.pattern"
	addPattern := "additional.pattern"
	metric := "super.puper.metric"
	addMetric := "additional.metric"
	addMetric2 := "additional.metric2"
	metricValues := []*moira.MetricValue{
		{
			RetentionTimestamp: 20,
			Timestamp:          23,
			Value:              0,
		},
		{
			RetentionTimestamp: 30,
			Timestamp:          33,
			Value:              1,
		},
		{
			RetentionTimestamp: 40,
			Timestamp:          43,
			Value:              2,
		},
		{
			RetentionTimestamp: 50,
			Timestamp:          53,
			Value:              3,
		},
		{
			RetentionTimestamp: 60,
			Timestamp:          63,
			Value:              4,
		},
	}
	dataList := map[string][]*moira.MetricValue{
		metric: metricValues,
	}

	var (
		from       int64 = 17
		until      int64 = 67
		metricsTTL int64 = 10
		retention  int64 = 10
		metricErr        = fmt.Errorf("Ooops, metric error")
	)

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerChecker := &TriggerChecker{
		Config: &Config{
			MetricsTTLSeconds: metricsTTL,
		},
		Database: dataBase,
		From:     from,
		Until:    until,
		logger:   test_helpers.GetTestLogger(),
		trigger: &moira.Trigger{
			Targets:  []string{pattern},
			Patterns: []string{pattern},
		},
	}

	Convey("Error test", t, func() {
		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, from, until).Return(nil, metricErr)
		dataBase.EXPECT().RemoveMetricsValues([]string{metric}, until-metricsTTL)
		actual, metrics, err := triggerChecker.getTimeSeries(from, until)
		So(actual, ShouldBeNil)
		So(metrics, ShouldBeNil)
		So(err, ShouldBeError)
		So(err, ShouldResemble, metricErr)
	})

	Convey("Test no metrics", t, func() {
		Convey("in main target", func() {
			dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{}, nil)
			actual, metrics, err := triggerChecker.getTimeSeries(from, until)
			timeSeries := target.TimeSeries{
				MetricData: et.MetricData{FetchResponse: pb.FetchResponse{
					Name:      pattern,
					StartTime: int32(from),
					StopTime:  int32(until),
					StepTime:  60,
					Values:    []float64{},
					IsAbsent:  []bool{},
				}},
				Wildcard: true,
			}
			expected := &triggerTimeSeries{
				Main:       []*target.TimeSeries{&timeSeries},
				Additional: make([]*target.TimeSeries, 0),
			}
			So(actual, ShouldResemble, expected)
			So(metrics, ShouldBeEmpty)
			So(err, ShouldBeNil)
		})
	})

	Convey("Test has metrics", t, func() {
		Convey("Only one target", func() {
			dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
			dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
			dataBase.EXPECT().GetMetricsValues([]string{metric}, from, until).Return(dataList, nil)
			actual, metrics, err := triggerChecker.getTimeSeries(from, until)
			fetchResponse := pb.FetchResponse{
				Name:      metric,
				StartTime: int32(from),
				StopTime:  int32(until),
				StepTime:  int32(retention),
				Values:    []float64{0, 1, 2, 3, 4},
				IsAbsent:  make([]bool, 5),
			}
			expected := &triggerTimeSeries{
				Main:       []*target.TimeSeries{{MetricData: et.MetricData{FetchResponse: fetchResponse}}},
				Additional: make([]*target.TimeSeries, 0),
			}
			So(err, ShouldBeNil)
			So(actual, ShouldResemble, expected)
			So(metrics, ShouldResemble, []string{metric})
		})

		Convey("Two targets", func() {
			triggerChecker.trigger.Targets = []string{pattern, addPattern}
			triggerChecker.trigger.Patterns = []string{pattern, addPattern}
			dataList[addMetric] = metricValues

			dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
			dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
			dataBase.EXPECT().GetMetricsValues([]string{metric}, from, until).Return(dataList, nil)

			dataBase.EXPECT().GetPatternMetrics(addPattern).Return([]string{addMetric}, nil)
			dataBase.EXPECT().GetMetricRetention(addMetric).Return(retention, nil)
			dataBase.EXPECT().GetMetricsValues([]string{addMetric}, from, until).Return(dataList, nil)

			dataBase.EXPECT().RemoveMetricsValues([]string{metric, addMetric}, until-metricsTTL)

			actual, metrics, err := triggerChecker.getTimeSeries(from, until)
			fetchResponse := pb.FetchResponse{
				Name:      metric,
				StartTime: int32(from),
				StopTime:  int32(until),
				StepTime:  int32(retention),
				Values:    []float64{0, 1, 2, 3},
				IsAbsent:  make([]bool, 4),
			}
			addFetchResponse := fetchResponse
			addFetchResponse.Name = addMetric
			expected := &triggerTimeSeries{
				Main:       []*target.TimeSeries{{MetricData: et.MetricData{FetchResponse: fetchResponse}}},
				Additional: []*target.TimeSeries{{MetricData: et.MetricData{FetchResponse: addFetchResponse}}},
			}

			So(err, ShouldBeNil)
			So(actual, ShouldResemble, expected)
			So(metrics, ShouldResemble, []string{metric, addMetric})
		})

		Convey("Two targets with many metrics in additional target", func() {
			dataList[addMetric2] = metricValues

			dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
			dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
			dataBase.EXPECT().GetMetricsValues([]string{metric}, from, until).Return(dataList, nil)

			dataBase.EXPECT().GetPatternMetrics(addPattern).Return([]string{addMetric, addMetric2}, nil)
			dataBase.EXPECT().GetMetricRetention(addMetric).Return(retention, nil)
			dataBase.EXPECT().GetMetricsValues([]string{addMetric, addMetric2}, from, until).Return(dataList, nil)

			actual, metrics, err := triggerChecker.getTimeSeries(from, until)
			So(err, ShouldBeError)
			So(err, ShouldResemble, ErrWrongTriggerTarget(2))
			So(err.Error(), ShouldResemble, "Target t2 has more than one timeseries")
			So(actual, ShouldBeNil)
			So(metrics, ShouldBeNil)
		})
	})
}

func TestGetTargetName(t *testing.T) {
	tts := triggerTimeSeries{}

	Convey("GetMainTargetName", t, func() {
		So(tts.getMainTargetName(), ShouldResemble, "t1")
	})

	Convey("GetAdditionalTargetName", t, func() {
		for i := 0; i < 5; i++ {
			So(tts.getAdditionalTargetName(i), ShouldResemble, fmt.Sprintf("t%v", i+2))
		}
	})
}

func TestGetExpressionValues(t *testing.T) {
	Convey("Has only main timeSeries", t, func() {
		fetchResponse := pb.FetchResponse{
			Name:      "m",
			StartTime: int32(17),
			StopTime:  int32(67),
			StepTime:  int32(10),
			Values:    []float64{0.0, 1.0, 2.0, 3.0, 4.0},
			IsAbsent:  []bool{false, true, true, false, true},
		}
		timeSeries := target.TimeSeries{
			MetricData: et.MetricData{FetchResponse: fetchResponse},
		}
		tts := &triggerTimeSeries{
			Main: []*target.TimeSeries{&timeSeries},
		}
		expectedExpressionValues := &expression.TriggerExpression{
			AdditionalTargetsValues: make(map[string]float64),
		}

		values, noEmptyValues := tts.getExpressionValues(&timeSeries, 17)
		So(noEmptyValues, ShouldBeTrue)
		So(values, ShouldResemble, expectedExpressionValues)

		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 67)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 11)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 44)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		expectedExpressionValues.MainTargetValue = 3
		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 53)
		So(noEmptyValues, ShouldBeTrue)
		So(values, ShouldResemble, expectedExpressionValues)
	})

	Convey("Has additional series", t, func() {
		fetchResponse := pb.FetchResponse{
			Name:      "main",
			StartTime: int32(17),
			StopTime:  int32(67),
			StepTime:  int32(10),
			Values:    []float64{0.0, 1.0, 2.0, 3.0, 4.0},
			IsAbsent:  []bool{false, true, true, false, true},
		}
		timeSeries := target.TimeSeries{
			MetricData: et.MetricData{FetchResponse: fetchResponse},
		}
		fetchResponseAdd := pb.FetchResponse{
			Name:      "main",
			StartTime: int32(17),
			StopTime:  int32(67),
			StepTime:  int32(10),
			Values:    []float64{4.0, 3.0, 2.0, 1.0, 0.0},
			IsAbsent:  []bool{false, false, true, true, false},
		}
		timeSeriesAdd := target.TimeSeries{
			MetricData: et.MetricData{FetchResponse: fetchResponseAdd},
		}
		tts := &triggerTimeSeries{
			Main:       []*target.TimeSeries{&timeSeries},
			Additional: []*target.TimeSeries{&timeSeriesAdd},
		}

		expectedExpressionValues := &expression.TriggerExpression{
			AdditionalTargetsValues: make(map[string]float64),
		}

		values, noEmptyValues := tts.getExpressionValues(&timeSeries, 29)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 42)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 65)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		expectedExpressionValues.MainTargetValue = 3
		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 50)
		So(noEmptyValues, ShouldBeFalse)
		So(values, ShouldResemble, expectedExpressionValues)

		expectedExpressionValues.MainTargetValue = 0
		expectedExpressionValues.AdditionalTargetsValues["t2"] = 4
		values, noEmptyValues = tts.getExpressionValues(&timeSeries, 17)
		So(noEmptyValues, ShouldBeTrue)
		So(values, ShouldResemble, expectedExpressionValues)
	})
}

func TestTriggerTimeSeriesHasOnlyWildcards(t *testing.T) {
	Convey("Main timeseries has wildcards only", t, func() {
		tts := triggerTimeSeries{
			Main: []*target.TimeSeries{{Wildcard: true}},
		}
		So(tts.hasOnlyWildcards(), ShouldBeTrue)

		tts1 := triggerTimeSeries{
			Main: []*target.TimeSeries{{Wildcard: true}, {Wildcard: true}},
		}
		So(tts1.hasOnlyWildcards(), ShouldBeTrue)
	})

	Convey("Main timeseries has not only wildcards", t, func() {
		tts := triggerTimeSeries{
			Main: []*target.TimeSeries{{Wildcard: false}},
		}
		So(tts.hasOnlyWildcards(), ShouldBeFalse)

		tts1 := triggerTimeSeries{
			Main: []*target.TimeSeries{{Wildcard: false}, {Wildcard: true}},
		}
		So(tts1.hasOnlyWildcards(), ShouldBeFalse)

		tts2 := triggerTimeSeries{
			Main: []*target.TimeSeries{{Wildcard: false}, {Wildcard: false}},
		}
		So(tts2.hasOnlyWildcards(), ShouldBeFalse)
	})

	Convey("Additional timeseries has wildcards but Main not", t, func() {
		tts := triggerTimeSeries{
			Main:       []*target.TimeSeries{{Wildcard: false}},
			Additional: []*target.TimeSeries{{Wildcard: true}, {Wildcard: true}},
		}
		So(tts.hasOnlyWildcards(), ShouldBeFalse)
	})
}
