package checker

import (
	"fmt"
	"math"
	"testing"
	"time"

	pb "github.com/go-graphite/carbonapi/carbonzipperpb3"
	et "github.com/go-graphite/carbonapi/expr/types"
	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/silencer"
	"go.avito.ru/DO/moira/target"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestGetTimeSeriesState(t *testing.T) {
	var (
		warnValue float64 = 10
		errValue  float64 = 20
	)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	test_helpers.InitTestLogging()

	triggerChecker := TriggerChecker{
		From:     17,
		Until:    67,
		logger:   test_helpers.GetTestLogger(),
		silencer: silencer.NewSilencer(dataBase, nil),
		Statsd:   metrics.NewCheckerMetrics(),
		trigger: &moira.Trigger{
			WarnValue:  &warnValue,
			ErrorValue: &errValue,
		},
	}
	fetchResponse := pb.FetchResponse{
		Name:      "main.metric",
		StartTime: int32(triggerChecker.From),
		StopTime:  int32(triggerChecker.Until),
		StepTime:  int32(10),
		Values:    []float64{1, 2, 3, 4, math.NaN()},
		IsAbsent:  []bool{false, true, false, false, false},
	}
	addFetchResponse := pb.FetchResponse{
		Name:      "additional.metric",
		StartTime: int32(triggerChecker.From),
		StopTime:  int32(triggerChecker.Until),
		StepTime:  int32(10),
		Values:    []float64{math.NaN(), 4, 3, 2, 1},
		IsAbsent:  []bool{false, false, false, false, false},
	}
	addFetchResponse.Name = "additional.metric"
	tts := &triggerTimeSeries{
		Main: []*target.TimeSeries{{
			MetricData: et.MetricData{FetchResponse: fetchResponse},
		}},
		Additional: []*target.TimeSeries{{
			MetricData: et.MetricData{FetchResponse: addFetchResponse},
		}},
	}
	metricLastState := &moira.MetricState{
		Suppressed: true,
	}

	Convey("Checkpoint more than valueTimestamp", t, func() {
		metricState, err := triggerChecker.getTimeSeriesState(tts, tts.Main[0], metricLastState, 37, 47)
		So(err, ShouldBeNil)
		So(metricState, ShouldBeNil)
	})

	Convey("Checkpoint lover than valueTimestamp", t, func() {
		Convey("Has all value by eventTimestamp step", func() {
			metricState, err := triggerChecker.getTimeSeriesState(tts, tts.Main[0], metricLastState, 42, 27)
			So(err, ShouldBeNil)
			So(metricState, ShouldResemble, &moira.MetricState{
				State:          moira.OK,
				Timestamp:      42,
				Value:          &fetchResponse.Values[2],
				Suppressed:     metricLastState.Suppressed,
				EventTimestamp: 0,
			})
		})

		Convey("No value in main timeSeries by eventTimestamp step", func() {
			metricState, err := triggerChecker.getTimeSeriesState(tts, tts.Main[0], metricLastState, 66, 11)
			So(err, ShouldBeNil)
			So(metricState, ShouldBeNil)
		})

		Convey("IsAbsent in main timeSeries by eventTimestamp step", func() {
			metricState, err := triggerChecker.getTimeSeriesState(tts, tts.Main[0], metricLastState, 29, 11)
			So(err, ShouldBeNil)
			So(metricState, ShouldBeNil)
		})

		Convey("No value in additional timeSeries by eventTimestamp step", func() {
			metricState, err := triggerChecker.getTimeSeriesState(tts, tts.Main[0], metricLastState, 26, 11)
			So(err, ShouldBeNil)
			So(metricState, ShouldBeNil)
		})
	})

	Convey("No warn and error value with default expression", t, func() {
		triggerChecker.trigger.WarnValue = nil
		triggerChecker.trigger.ErrorValue = nil
		metricState, err := triggerChecker.getTimeSeriesState(tts, tts.Main[0], metricLastState, 42, 27)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldResemble, "Error value and Warning value can not be empty")
		So(metricState, ShouldBeNil)
	})
}

func TestGetTimeSeriesStepsStates(t *testing.T) {
	var (
		warnValue float64 = 10
		errValue  float64 = 20
	)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	test_helpers.InitTestLogging()

	triggerChecker := TriggerChecker{
		Until:    67,
		From:     17,
		logger:   test_helpers.GetTestLogger(),
		silencer: silencer.NewSilencer(dataBase, nil),
		trigger: &moira.Trigger{
			WarnValue:  &warnValue,
			ErrorValue: &errValue,
		},
	}
	fetchResponse1 := pb.FetchResponse{
		Name:      "main.metric",
		StartTime: int32(triggerChecker.From),
		StopTime:  int32(triggerChecker.Until),
		StepTime:  int32(10),
		Values:    []float64{1, 2, 3, 4, math.NaN()},
		IsAbsent:  []bool{false, true, false, false, false},
	}
	fetchResponse2 := pb.FetchResponse{
		Name:      "main.metric",
		StartTime: int32(triggerChecker.From),
		StopTime:  int32(triggerChecker.Until),
		StepTime:  int32(10),
		Values:    []float64{1, 2, 3, 4, 5},
		IsAbsent:  []bool{false, false, false, false, false},
	}
	addFetchResponse := pb.FetchResponse{
		Name:      "additional.metric",
		StartTime: int32(triggerChecker.From),
		StopTime:  int32(triggerChecker.Until),
		StepTime:  int32(10),
		Values:    []float64{5, 4, 3, 2, 1},
		IsAbsent:  []bool{false, false, false, false, false},
	}
	addFetchResponse.Name = "additional.metric"
	tts := &triggerTimeSeries{
		Main:       []*target.TimeSeries{{MetricData: et.MetricData{FetchResponse: fetchResponse1}}, {MetricData: et.MetricData{FetchResponse: fetchResponse2}}},
		Additional: []*target.TimeSeries{{MetricData: et.MetricData{FetchResponse: addFetchResponse}}},
	}
	metricLastState := &moira.MetricState{
		Suppressed:     true,
		EventTimestamp: 11,
	}

	metricsState1 := &moira.MetricState{
		State:          moira.OK,
		Timestamp:      17,
		Value:          &fetchResponse2.Values[0],
		Suppressed:     metricLastState.Suppressed,
		EventTimestamp: 0,
	}

	metricsState2 := &moira.MetricState{
		State:          moira.OK,
		Timestamp:      27,
		Value:          &fetchResponse2.Values[1],
		Suppressed:     metricLastState.Suppressed,
		EventTimestamp: 0,
	}

	metricsState3 := &moira.MetricState{
		State:          moira.OK,
		Timestamp:      37,
		Value:          &fetchResponse2.Values[2],
		Suppressed:     metricLastState.Suppressed,
		EventTimestamp: 0,
	}

	metricsState4 := &moira.MetricState{
		State:          moira.OK,
		Timestamp:      47,
		Value:          &fetchResponse2.Values[3],
		Suppressed:     metricLastState.Suppressed,
		EventTimestamp: 0,
	}

	metricsState5 := &moira.MetricState{
		State:          moira.OK,
		Timestamp:      57,
		Value:          &fetchResponse2.Values[4],
		Suppressed:     metricLastState.Suppressed,
		EventTimestamp: 0,
	}

	Convey("ValueTimestamp covers all TimeSeries range", t, func() {
		metricLastState.EventTimestamp = 11
		Convey("TimeSeries has all valid values", func() {
			metricStates, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[1], metricLastState)
			So(err, ShouldBeNil)
			So(metricStates, ShouldResemble, []*moira.MetricState{metricsState1, metricsState2, metricsState3, metricsState4, metricsState5})
		})

		Convey("TimeSeries has invalid values", func() {
			metricStates, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[0], metricLastState)
			So(err, ShouldBeNil)
			So(metricStates, ShouldResemble, []*moira.MetricState{metricsState1, metricsState3, metricsState4})
		})

		Convey("Until + stepTime covers last value", func() {
			triggerChecker.Until = 56
			metricStates, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[1], metricLastState)
			So(err, ShouldBeNil)
			So(metricStates, ShouldResemble, []*moira.MetricState{metricsState1, metricsState2, metricsState3, metricsState4, metricsState5})
		})
	})

	triggerChecker.Until = 67

	Convey("ValueTimestamp don't covers begin of TimeSeries", t, func() {
		Convey("Exclude 1 first element", func() {
			metricLastState.EventTimestamp = 22
			metricStates, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[1], metricLastState)
			So(err, ShouldBeNil)
			So(metricStates, ShouldResemble, []*moira.MetricState{metricsState2, metricsState3, metricsState4, metricsState5})
		})

		Convey("Exclude 2 first elements", func() {
			metricLastState.EventTimestamp = 27
			metricStates, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[1], metricLastState)
			So(err, ShouldBeNil)
			So(metricStates, ShouldResemble, []*moira.MetricState{metricsState3, metricsState4, metricsState5})
		})

		Convey("Exclude last element", func() {
			metricLastState.EventTimestamp = 11
			triggerChecker.Until = 47
			metricStates, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[1], metricLastState)
			So(err, ShouldBeNil)
			So(metricStates, ShouldResemble, []*moira.MetricState{metricsState1, metricsState2, metricsState3, metricsState4})
		})
	})

	Convey("No warn and error value with default expression", t, func() {
		metricLastState.EventTimestamp = 11
		triggerChecker.Until = 47
		triggerChecker.trigger.WarnValue = nil
		triggerChecker.trigger.ErrorValue = nil
		metricState, err := triggerChecker.getTimeSeriesStepsStates(tts, tts.Main[1], metricLastState)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldResemble, "Error value and Warning value can not be empty")
		So(metricState, ShouldBeNil)
	})
}

func TestCheckForNODATA(t *testing.T) {
	metricLastState := &moira.MetricState{
		EventTimestamp: 11,
		Suppressed:     true,
	}
	Convey("No TTL", t, func() {
		triggerChecker := TriggerChecker{}
		currentState, deleteMetrics := triggerChecker.checkForNoData(metricLastState)
		So(deleteMetrics, ShouldBeFalse)
		So(currentState, ShouldBeNil)
	})

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	test_helpers.InitTestLogging()

	var ttl int64 = 600
	triggerChecker := TriggerChecker{
		Statsd:   metrics.NewCheckerMetrics(),
		silencer: silencer.NewSilencer(dataBase, nil),
		ttl:      ttl,
		lastCheck: &moira.CheckData{
			Timestamp: 1000,
		},
	}

	Convey("Last check is resent", t, func() {
		Convey("1", func() {
			metricLastState.Timestamp = 1100
			currentState, deleteMetrics := triggerChecker.checkForNoData(metricLastState)
			So(deleteMetrics, ShouldBeFalse)
			So(currentState, ShouldBeNil)
		})
		Convey("2", func() {
			metricLastState.Timestamp = 401
			currentState, deleteMetrics := triggerChecker.checkForNoData(metricLastState)
			So(deleteMetrics, ShouldBeFalse)
			So(currentState, ShouldBeNil)
		})
	})

	metricLastState.Timestamp = 399
	triggerChecker.ttlState = moira.DEL

	Convey("TTLState is DEL and has EventTimeStamp", t, func() {
		currentState, deleteMetric := triggerChecker.checkForNoData(metricLastState)
		So(deleteMetric, ShouldBeTrue)
		So(currentState, ShouldBeNil)
	})

	Convey("Has new metricState", t, func() {
		Convey("TTLState is DEL, but no EventTimestamp", func() {
			metricLastState.EventTimestamp = 0
			currentState, deleteMetric := triggerChecker.checkForNoData(metricLastState)
			So(deleteMetric, ShouldBeFalse)
			So(currentState, ShouldResemble, &moira.MetricState{
				State:      moira.NODATA,
				Timestamp:  triggerChecker.CheckStarted,
				Value:      nil,
				Suppressed: metricLastState.Suppressed,
				IsNoData:   true,
			})
		})

		Convey("TTLState is OK and no EventTimestamp", func() {
			metricLastState.EventTimestamp = 0
			triggerChecker.ttlState = moira.OK
			currentState, deleteMetric := triggerChecker.checkForNoData(metricLastState)
			So(deleteMetric, ShouldBeFalse)
			So(currentState, ShouldResemble, &moira.MetricState{
				State:      triggerChecker.ttlState,
				Timestamp:  triggerChecker.CheckStarted,
				Value:      nil,
				Suppressed: metricLastState.Suppressed,
				IsNoData:   true,
			})
		})

		Convey("TTLState is OK and has EventTimestamp", func() {
			metricLastState.EventTimestamp = 111
			currentState, deleteMetric := triggerChecker.checkForNoData(metricLastState)
			So(deleteMetric, ShouldBeFalse)
			So(currentState, ShouldResemble, &moira.MetricState{
				State:      triggerChecker.ttlState,
				Timestamp:  triggerChecker.CheckStarted,
				Value:      nil,
				Suppressed: metricLastState.Suppressed,
				IsNoData:   true,
			})
		})
	})
}

func TestCheckErrors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	test_helpers.InitTestLogging()

	var retention int64 = 10
	pattern := "super.puper.pattern"
	metric := "super.puper.metric"
	metricErr := fmt.Errorf("Ooops, metric error")

	var ttl int64 = 30

	triggerChecker := TriggerChecker{
		TriggerID: "SuperId",
		Config: &Config{
			MetricsTTLSeconds: 10,
		},
		Database: dataBase,
		Statsd:   metrics.NewCheckerMetrics(),
		From:     17,
		Until:    67,
		logger:   test_helpers.GetTestLogger(),
		silencer: silencer.NewSilencer(dataBase, nil),
		ttl:      ttl,
		ttlState: moira.NODATA,
		trigger: &moira.Trigger{
			Targets:  []string{pattern},
			Patterns: []string{pattern},
		},
		lastCheck: &moira.CheckData{
			State:     moira.EXCEPTION,
			Timestamp: 57,
			Metrics: map[string]*moira.MetricState{
				metric: {
					State:     moira.OK,
					Timestamp: 26,
				},
			},
		},
	}

	Convey("GetTimeSeries error", t, func() {
		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, triggerChecker.From, triggerChecker.Until).Return(nil, metricErr)
		dataBase.EXPECT().SetTriggerLastCheck(triggerChecker.TriggerID, &moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics:           triggerChecker.lastCheck.Metrics,
			State:             moira.EXCEPTION,
			Timestamp:         triggerChecker.Until,
			EventTimestamp:    triggerChecker.Until,
			Score:             100000,
			Message:           metricErr.Error(),
		}).Return(nil)

		err := triggerChecker.Check()
		So(err, ShouldBeNil)
	})
}

func TestHandleTrigger(t *testing.T) {
	var (
		errValue  float64 = 20
		warnValue float64 = 10
		retention int64   = 10
		ttl       int64   = 600
	)

	pattern := "super.puper.pattern"
	metric := "super.puper.metric"

	lastCheck := moira.CheckData{
		MaintenanceMetric: make(map[string]int64),
		Metrics:           make(map[string]*moira.MetricState),
		State:             moira.NODATA,
		Timestamp:         66,
	}
	metricValues := []*moira.MetricValue{
		{
			RetentionTimestamp: 3620,
			Timestamp:          3623,
			Value:              0,
		},
		{
			RetentionTimestamp: 3630,
			Timestamp:          3633,
			Value:              1,
		},
		{
			RetentionTimestamp: 3640,
			Timestamp:          3643,
			Value:              2,
		},
		{
			RetentionTimestamp: 3650,
			Timestamp:          3653,
			Value:              3,
		},
		{
			RetentionTimestamp: 3660,
			Timestamp:          3663,
			Value:              4,
		},
	}
	dataList := map[string][]*moira.MetricValue{
		metric: metricValues,
	}

	makeTriggerChecker := func(dataBase *mock_moira_alert.MockDatabase) TriggerChecker {
		lastCheckCopy := lastCheck
		return TriggerChecker{
			TriggerID: "SuperId",
			Config: &Config{
				MetricsTTLSeconds: 3600,
			},
			Database:  dataBase,
			From:      3617,
			Until:     3667,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			lastCheck: &lastCheckCopy,
			ttl:       ttl,
			ttlState:  moira.NODATA,
			trigger: &moira.Trigger{
				ErrorValue: &errValue,
				WarnValue:  &warnValue,
				Targets:    []string{pattern},
				Patterns:   []string{pattern},
			},
		}
	}

	Convey("First Event", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		triggerChecker := makeTriggerChecker(dataBase)

		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, triggerChecker.From, triggerChecker.Until).Return(dataList, nil)
		var val float64
		var val1 float64 = 4
		dataBase.EXPECT().RemoveMetricsValues([]string{metric}, triggerChecker.Until-triggerChecker.Config.MetricsTTLSeconds)
		dataBase.EXPECT().PushNotificationEvent(&moira.NotificationEvent{
			TriggerID: triggerChecker.TriggerID,
			Timestamp: 3617,
			State:     moira.OK,
			OldState:  moira.NODATA,
			Metric:    metric,
			Value:     &val,
			Message:   nil,
		}).Return(nil)
		dataBase.EXPECT().GetChildEvents(triggerChecker.TriggerID, metric).Return(nil, nil)
		checkData, err := triggerChecker.handleTrigger()
		So(err, ShouldBeNil)
		So(checkData, ShouldResemble, moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics: map[string]*moira.MetricState{
				metric: {
					Timestamp:      3657,
					EventTimestamp: 3617,
					State:          moira.OK,
					Value:          &val1,
				},
			},
			Timestamp: triggerChecker.Until,
			State:     moira.OK,
			Score:     0,
		})
	})

	var (
		val  float64 = 3
		val1 float64 = 4
	)

	lastCheck = moira.CheckData{
		Metrics: map[string]*moira.MetricState{
			metric: {
				Timestamp:      3647,
				EventTimestamp: 3607,
				State:          moira.OK,
				Value:          &val,
			},
		},
		State:     moira.OK,
		Timestamp: 3655,
	}

	Convey("Last check is not empty", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		triggerChecker := makeTriggerChecker(dataBase)

		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, triggerChecker.From, triggerChecker.Until).Return(dataList, nil)
		dataBase.EXPECT().RemoveMetricsValues([]string{metric}, triggerChecker.Until-triggerChecker.Config.MetricsTTLSeconds)
		checkData, err := triggerChecker.handleTrigger()
		So(err, ShouldBeNil)
		So(checkData, ShouldResemble, moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics: map[string]*moira.MetricState{
				metric: {
					Timestamp:      3657,
					EventTimestamp: 3607,
					State:          moira.OK,
					Value:          &val1,
				},
			},
			Timestamp: triggerChecker.Until,
			State:     moira.OK,
			Score:     0,
		})
	})

	Convey("No data too long", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		lastCheck.Timestamp = 4267

		triggerChecker := makeTriggerChecker(dataBase)
		triggerChecker.CheckStarted = lastCheck.Timestamp
		triggerChecker.From = 4217
		triggerChecker.Until = 4267
		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, triggerChecker.From, triggerChecker.Until).Return(dataList, nil)
		dataBase.EXPECT().RemoveMetricsValues([]string{metric}, triggerChecker.Until-triggerChecker.Config.MetricsTTLSeconds)
		dataBase.EXPECT().PushNotificationEvent(&moira.NotificationEvent{
			TriggerID: triggerChecker.TriggerID,
			Timestamp: lastCheck.Timestamp,
			State:     moira.NODATA,
			OldState:  moira.OK,
			Metric:    metric,
			Value:     nil,
			OldValue:  &val1,
			Message:   nil,
		}).Return(nil)

		checkData, err := triggerChecker.handleTrigger()
		So(err, ShouldBeNil)
		So(checkData, ShouldResemble, moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics: map[string]*moira.MetricState{
				metric: {
					IsNoData:       true,
					Timestamp:      lastCheck.Timestamp,
					EventTimestamp: lastCheck.Timestamp,
					State:          moira.NODATA,
					Value:          nil,
				},
			},
			Timestamp: triggerChecker.Until,
			State:     moira.OK,
			Score:     0,
		})
	})

	Convey("No metrics, should return trigger has only wildcards error", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		triggerChecker := makeTriggerChecker(dataBase)

		triggerChecker.From = 4217
		triggerChecker.Until = 4267
		triggerChecker.ttlState = moira.NODATA
		lastCheck.Timestamp = 4267
		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{}, nil)
		checkData, err := triggerChecker.handleTrigger()
		So(err, ShouldResemble, ErrTriggerHasOnlyWildcards{})
		So(checkData, ShouldResemble, moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics:           lastCheck.Metrics,
			Timestamp:         triggerChecker.Until,
			State:             moira.OK,
			Score:             0,
		})
	})

	Convey("Has duplicated names timeseries, should return trigger has same timeseries names error", t, func() {
		metric1 := "super.puper.metric"
		metric2 := "super.drupper.metric"
		pattern1 := "super.*.metric"
		f := 3.0

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker1 := TriggerChecker{
			TriggerID: "SuperId",
			Config: &Config{
				MetricsTTLSeconds: 3600,
			},
			Database: dataBase,
			From:     3617,
			Until:    3667,
			logger:   test_helpers.GetTestLogger(),
			silencer: silencer.NewSilencer(dataBase, nil),
			ttl:      ttl,
			ttlState: moira.NODATA,
			trigger: &moira.Trigger{
				ErrorValue: &errValue,
				WarnValue:  &warnValue,
				Targets:    []string{"aliasByNode(super.*.metric, 0)"},
				Patterns:   []string{pattern1},
			},
			lastCheck: &moira.CheckData{
				MaintenanceMetric: map[string]int64{},
				Metrics:           make(map[string]*moira.MetricState),
				State:             moira.NODATA,
				Timestamp:         3647,
			},
		}
		dataBase.EXPECT().GetPatternMetrics(pattern1).Return([]string{metric1, metric2}, nil)
		dataBase.EXPECT().GetMetricRetention(metric1).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric1, metric2}, triggerChecker1.From, triggerChecker1.Until).Return(map[string][]*moira.MetricValue{metric1: metricValues, metric2: metricValues}, nil)
		dataBase.EXPECT().RemoveMetricsValues([]string{metric1, metric2}, gomock.Any())
		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(nil)
		dataBase.EXPECT().GetChildEvents(triggerChecker1.TriggerID, "super").Return(nil, nil)
		checkData, err := triggerChecker1.handleTrigger()
		So(err, ShouldResemble, ErrTriggerHasSameTimeSeriesNames{})
		So(checkData, ShouldResemble, moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics: map[string]*moira.MetricState{
				"super": {
					EventTimestamp: 3617,
					State:          moira.OK,
					Suppressed:     false,
					Timestamp:      3647,
					Value:          &f,
				},
			},
			Score:          0,
			State:          moira.OK,
			Timestamp:      3667,
			EventTimestamp: 0,
			Suppressed:     false,
			Message:        "",
		})
	})

	Convey("No data too long and ttlState is delete", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		lastCheck.Timestamp = 3657
		lastCheck.Metrics[metric] = &moira.MetricState{
			IsNoData:       true,
			State:          moira.NODATA,
			EventTimestamp: lastCheck.Timestamp,
			Timestamp:      lastCheck.Timestamp,
		}

		triggerChecker := makeTriggerChecker(dataBase)
		triggerChecker.From = 4217
		triggerChecker.Until = 4267
		triggerChecker.lastCheck.Timestamp = 4267
		triggerChecker.ttlState = moira.DEL

		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, triggerChecker.From, triggerChecker.Until).Return(dataList, nil)
		dataBase.EXPECT().RemoveMetricsValues([]string{metric}, triggerChecker.Until-triggerChecker.Config.MetricsTTLSeconds)
		dataBase.EXPECT().RemovePatternsMetrics(triggerChecker.trigger.Patterns).Return(nil)
		checkData, err := triggerChecker.handleTrigger()
		So(err, ShouldBeNil)
		So(checkData, ShouldResemble, moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics:           make(map[string]*moira.MetricState),
			Timestamp:         triggerChecker.Until,
			State:             moira.OK,
			Score:             0,
		})
	})
}

func TestHandleErrorCheck(t *testing.T) {
	Convey("Handle error no metrics", t, func() {
		Convey("TTL is 0", func() {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
			triggerChecker := TriggerChecker{
				TriggerID: "SuperId",
				Database:  dataBase,
				logger:    test_helpers.GetTestLogger(),
				silencer:  silencer.NewSilencer(dataBase, nil),
				ttl:       0,
				ttlState:  moira.NODATA,
				trigger:   &moira.Trigger{},
				lastCheck: &moira.CheckData{
					Timestamp: 0,
					State:     moira.NODATA,
				},
			}
			checkData := moira.CheckData{
				State:     moira.NODATA,
				Timestamp: time.Now().Unix(),
				Message:   "Trigger has no metrics, check your target",
			}

			actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasNoTimeSeries{})
			So(err, ShouldBeNil)
			So(actual, ShouldResemble, checkData)
		})

		Convey("TTL is not 0", func() {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
			test_helpers.InitTestLogging()

			triggerChecker := TriggerChecker{
				TriggerID: "SuperId",
				Database:  dataBase,
				logger:    test_helpers.GetTestLogger(),
				silencer:  silencer.NewSilencer(dataBase, nil),
				ttl:       60,
				trigger:   &moira.Trigger{},
				ttlState:  moira.NODATA,
				lastCheck: &moira.CheckData{
					Timestamp: 0,
					State:     moira.NODATA,
				},
			}
			err1 := "This metric has been in bad state for more than 24 hours - please, fix."
			checkData := moira.CheckData{
				State:     moira.OK,
				Timestamp: time.Now().Unix(),
			}
			event := &moira.NotificationEvent{
				IsTriggerEvent: true,
				Timestamp:      checkData.Timestamp,
				Message:        &err1,
				TriggerID:      triggerChecker.TriggerID,
				OldState:       moira.NODATA,
				State:          moira.NODATA,
			}

			dataBase.EXPECT().PushNotificationEvent(event).Return(nil)
			actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasNoTimeSeries{})
			expected := moira.CheckData{
				State:          moira.NODATA,
				Timestamp:      checkData.Timestamp,
				EventTimestamp: checkData.Timestamp,
				Message:        "Trigger has no metrics, check your target",
			}
			So(err, ShouldBeNil)
			So(actual, ShouldResemble, expected)
		})
	})

	Convey("Handle trigger has only wildcards without metrics in last state", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger:   &moira.Trigger{},
			ttlState:  moira.ERROR,
			lastCheck: &moira.CheckData{
				Timestamp: time.Now().Unix(),
				State:     moira.OK,
			},
		}
		checkData := moira.CheckData{
			State:     moira.OK,
			Timestamp: time.Now().Unix(),
		}

		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(nil)
		actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasOnlyWildcards{})
		expected := moira.CheckData{
			State:          moira.NODATA,
			Timestamp:      checkData.Timestamp,
			EventTimestamp: checkData.Timestamp,
			Message:        "Trigger never received metrics",
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})

	Convey("Handle trigger has only wildcards with metrics in last state", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger:   &moira.Trigger{},
			ttlState:  moira.NODATA,
			lastCheck: &moira.CheckData{
				Timestamp: time.Now().Unix(),
				State:     moira.OK,
			},
		}
		checkData := moira.CheckData{
			Metrics: map[string]*moira.MetricState{
				"123": {},
			},
			State:     moira.OK,
			Timestamp: time.Now().Unix(),
		}

		actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasOnlyWildcards{})
		expected := moira.CheckData{
			Metrics:        checkData.Metrics,
			State:          moira.OK,
			Timestamp:      checkData.Timestamp,
			EventTimestamp: checkData.Timestamp,
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})

	Convey("Handle trigger has only wildcards and ttlState is OK", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger:   &moira.Trigger{},
			ttlState:  moira.OK,
			lastCheck: &moira.CheckData{
				Timestamp: time.Now().Unix(),
				State:     moira.OK,
			},
		}
		checkData := moira.CheckData{
			Metrics:   map[string]*moira.MetricState{},
			State:     moira.OK,
			Timestamp: time.Now().Unix(),
		}

		actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasOnlyWildcards{})
		expected := moira.CheckData{
			Metrics:        checkData.Metrics,
			State:          moira.OK,
			Timestamp:      checkData.Timestamp,
			EventTimestamp: checkData.Timestamp,
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})

	Convey("Handle trigger has only wildcards and ttlState is DEL", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		now := time.Now().Unix()
		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger:   &moira.Trigger{},
			ttlState:  moira.DEL,
			lastCheck: &moira.CheckData{
				Timestamp:      now,
				EventTimestamp: now - 3600,
				State:          moira.OK,
			},
		}
		checkData := moira.CheckData{
			Metrics:   map[string]*moira.MetricState{},
			State:     moira.OK,
			Timestamp: now,
		}

		actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasOnlyWildcards{})
		expected := moira.CheckData{
			Metrics:        checkData.Metrics,
			State:          moira.OK,
			Timestamp:      now,
			EventTimestamp: now - 3600,
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})

	Convey("Handle unknown function in evalExpr", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger:   &moira.Trigger{},
			ttlState:  moira.NODATA,
			lastCheck: &moira.CheckData{
				Timestamp: time.Now().Unix(),
				State:     moira.OK,
			},
		}
		checkData := moira.CheckData{
			State:     moira.OK,
			Timestamp: time.Now().Unix(),
		}

		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(nil)

		actual, err := triggerChecker.handleErrorCheck(checkData, target.ErrUnknownFunction{FuncName: "123"})
		expected := moira.CheckData{
			State:          moira.EXCEPTION,
			Timestamp:      checkData.Timestamp,
			EventTimestamp: checkData.Timestamp,
			Message:        "Unknown graphite function: \"123\"",
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})

	Convey("Handle trigger has same timeseries names", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger:   &moira.Trigger{},
			ttlState:  moira.NODATA,
			lastCheck: &moira.CheckData{
				Timestamp: time.Now().Unix(),
				State:     moira.OK,
			},
		}
		checkData := moira.CheckData{
			State:     moira.OK,
			Timestamp: time.Now().Unix(),
		}

		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(nil)

		actual, err := triggerChecker.handleErrorCheck(checkData, ErrTriggerHasSameTimeSeriesNames{})
		expected := moira.CheckData{
			State:          moira.EXCEPTION,
			Timestamp:      checkData.Timestamp,
			EventTimestamp: checkData.Timestamp,
			Message:        "Trigger has same timeseries names",
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})

	Convey("Handle additional trigger target has more than one timeseries", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		test_helpers.InitTestLogging()

		triggerChecker := TriggerChecker{
			TriggerID: "SuperId",
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
			ttl:       60,
			trigger: &moira.Trigger{
				Targets: []string{"aliasByNode(some.data.*,2)", "aliasByNode(some.more.data.*,2)"},
			},
			ttlState: moira.NODATA,
			lastCheck: &moira.CheckData{
				Timestamp: time.Now().Unix(),
				State:     moira.NODATA,
			},
		}
		checkData := moira.CheckData{
			State:     moira.NODATA,
			Timestamp: time.Now().Unix(),
		}

		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(nil)

		actual, err := triggerChecker.handleErrorCheck(checkData, ErrWrongTriggerTarget(2))
		expected := moira.CheckData{
			State:          moira.EXCEPTION,
			Timestamp:      checkData.Timestamp,
			EventTimestamp: checkData.Timestamp,
			Message:        "Target t2 has more than one timeseries",
		}
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, expected)
	})
}
