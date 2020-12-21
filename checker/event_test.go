package checker

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/satori/go.uuid"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/silencer"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestCompareStates(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerChecker := TriggerChecker{
		TriggerID: "SuperId",
		Database:  dataBase,
		logger:    test_helpers.GetTestLogger(),
		silencer:  silencer.NewSilencer(dataBase, nil),
		trigger:   &moira.Trigger{},
	}

	lastStateExample := &moira.MetricState{
		Timestamp:      1502712000,
		EventTimestamp: 1502708400,
		Suppressed:     false,
	}
	currentStateExample := &moira.MetricState{
		Suppressed: false,
		Timestamp:  1502719200,
		State:      moira.NODATA,
	}

	Convey("Same state values", t, func() {
		Convey("Status OK, no need to send", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.OK
			currentState.State = moira.OK

			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = lastState.EventTimestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})

		Convey("Status NODATA and no remind interval, no need to send", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.NODATA
			currentState.State = moira.NODATA

			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = lastState.EventTimestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})

		Convey("Status ERROR and no remind interval, no need to send", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.ERROR
			currentState.State = moira.ERROR

			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = lastState.EventTimestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})

		Convey("Status NODATA and remind interval, need to send", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.NODATA
			currentState.State = moira.NODATA
			currentState.Timestamp = 1502809200

			message := fmt.Sprintf("This metric has been in bad state for more than 24 hours - please, fix.")
			dataBase.EXPECT().PushNotificationEvent(&moira.NotificationEvent{
				TriggerID: triggerChecker.TriggerID,
				Timestamp: currentState.Timestamp,
				State:     moira.NODATA,
				OldState:  moira.NODATA,
				Metric:    "m1",
				Value:     currentState.Value,
				Message:   &message,
			}).Return(nil)
			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = currentState.Timestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})

		Convey("Status ERROR and remind interval, need to send", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.ERROR
			currentState.State = moira.ERROR
			currentState.Timestamp = 1502809200

			message := fmt.Sprintf("This metric has been in bad state for more than 24 hours - please, fix.")
			dataBase.EXPECT().PushNotificationEvent(&moira.NotificationEvent{
				TriggerID: triggerChecker.TriggerID,
				Timestamp: currentState.Timestamp,
				State:     moira.ERROR,
				OldState:  moira.ERROR,
				Metric:    "m1",
				Value:     currentState.Value,
				Message:   &message,
			}).Return(nil)
			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = currentState.Timestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})

		Convey("Status EXCEPTION and lastState.Suppressed=false", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.EXCEPTION
			currentState.State = moira.EXCEPTION

			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = lastState.EventTimestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})

		Convey("Status EXCEPTION and lastState.Suppressed=true", func() {
			lastState := lastStateExample
			currentState := currentStateExample
			lastState.State = moira.EXCEPTION
			lastState.Suppressed = true
			currentState.State = moira.EXCEPTION

			dataBase.EXPECT().PushNotificationEvent(&moira.NotificationEvent{
				TriggerID: triggerChecker.TriggerID,
				Timestamp: currentState.Timestamp,
				State:     moira.EXCEPTION,
				OldState:  moira.EXCEPTION,
				Metric:    "m1",
				Value:     currentState.Value,
				Message:   nil,
			}).Return(nil)

			actual, err := triggerChecker.compareStates("m1", currentState, lastState, false)
			So(err, ShouldBeNil)
			currentState.EventTimestamp = currentState.Timestamp
			currentState.Suppressed = false
			So(actual, ShouldResemble, currentState)
		})
	})
}

func TestCompareChecks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerChecker := TriggerChecker{
		TriggerID: "SuperId",
		Database:  dataBase,
		logger:    test_helpers.GetTestLogger(),
		silencer:  silencer.NewSilencer(dataBase, nil),
		trigger:   &moira.Trigger{},
	}

	lastCheckExample := moira.CheckData{
		Timestamp:      1502712000,
		EventTimestamp: 1502708400,
		Suppressed:     false,
	}
	currentCheckExample := moira.CheckData{
		Suppressed: false,
		Timestamp:  1502719200,
	}

	Convey("Same states", t, func() {
		Convey("No need send", func() {
			lastCheck := lastCheckExample
			currentCheck := currentCheckExample
			triggerChecker.lastCheck = &lastCheck
			lastCheck.State = moira.OK
			currentCheck.State = moira.OK
			actual, err := triggerChecker.compareChecks(currentCheck, false)

			So(err, ShouldBeNil)
			currentCheck.EventTimestamp = lastCheck.EventTimestamp
			So(actual, ShouldResemble, currentCheck)
		})

		Convey("Need send", func() {
			lastCheck := lastCheckExample
			currentCheck := currentCheckExample
			triggerChecker.lastCheck = &lastCheck
			lastCheck.State = moira.EXCEPTION
			lastCheck.Suppressed = true
			currentCheck.State = moira.EXCEPTION

			dataBase.EXPECT().PushNotificationEvent(&moira.NotificationEvent{
				IsTriggerEvent: true,
				TriggerID:      triggerChecker.TriggerID,
				Timestamp:      currentCheck.Timestamp,
				State:          moira.EXCEPTION,
				OldState:       moira.EXCEPTION,
				Metric:         triggerChecker.trigger.Name,
				Value:          nil,
				Message:        &currentCheck.Message,
			}).Return(nil)

			actual, err := triggerChecker.compareChecks(currentCheck, false)
			So(err, ShouldBeNil)
			currentCheck.EventTimestamp = currentCheck.Timestamp
			So(actual, ShouldResemble, currentCheck)
		})
	})

	Convey("Different states", t, func() {
		Convey("Schedule does not allows", func() {
			lastCheck := lastCheckExample
			lastCheck.State = moira.OK

			currentCheck := currentCheckExample
			currentCheck.State = moira.NODATA

			triggerChecker.lastCheck = &lastCheck
			triggerChecker.trigger.Schedule = test_helpers.GetScheduleNever()

			disabled := triggerChecker.isHandleMetricDisabled(currentCheck.Timestamp, moira.WildcardMetric, "")
			So(disabled, ShouldBeTrue)
		})
	})
}

func TestTTLMetricEvents(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ttlState := moira.ERROR
	triggerID := uuid.NewV4().String()

	lastCheckDelta := int64(24*60*60 - 5)
	triggerTTL := int64(3 * 60 * 60)

	trigger := &moira.Trigger{
		ID:              triggerID,
		TTL:             triggerTTL,
		TTLState:        &ttlState,
		PendingInterval: 60,
	}

	Convey("The first event isn't emitted", t, func() {
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		triggerChecker := TriggerChecker{
			TriggerID: triggerID,
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
		}
		lastCheck := &moira.CheckData{}
		lastCheckExpected := &moira.CheckData{}

		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(lastCheck, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)
		dataBase.EXPECT().SetTriggerLastCheck(triggerID, lastCheckExpected)

		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		lastCheck.Metrics = map[string]*moira.MetricState{
			"Metric 1": {
				Timestamp: triggerChecker.CheckStarted - lastCheckDelta,
				State:     moira.OK,
			},
		}

		lastCheckExpected.Metrics = lastCheck.Metrics
		lastCheckExpected.IsPending = true
		lastCheckExpected.EventTimestamp = triggerChecker.CheckStarted
		lastCheckExpected.Timestamp = triggerChecker.CheckStarted
		lastCheckExpected.Message = "Trigger has no metrics, check your target"
		lastCheckExpected.MaintenanceMetric = make(map[string]int64)
		lastCheckExpected.Metrics = map[string]*moira.MetricState{
			"Metric 1": {
				EventTimestamp: lastCheckExpected.EventTimestamp,
				// Timestamp hasn't changed because MetricState is pending
				Timestamp: triggerChecker.CheckStarted - lastCheckDelta,
				// neither has State
				State:     moira.OK,
				IsPending: true,
				IsNoData:  true,
			},
		}

		err = triggerChecker.Check()
		So(err, ShouldBeNil)
	})

	Convey("NODATA event is emitted after pending interval", t, func() {
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		triggerChecker := TriggerChecker{
			TriggerID: triggerID,
			Database:  dataBase,
			logger:    test_helpers.GetTestLogger(),
			silencer:  silencer.NewSilencer(dataBase, nil),
		}
		eventExpected := &moira.NotificationEvent{
			TriggerID: triggerID,
			Metric:    "Metric 1",
			State:     moira.ERROR,
			OldState:  moira.OK,
		}

		lastCheck := &moira.CheckData{}
		lastCheckExpected := &moira.CheckData{}

		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(lastCheck, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)
		dataBase.EXPECT().PushNotificationEvent(eventExpected) // event is emitted
		dataBase.EXPECT().SetTriggerLastCheck(triggerID, lastCheckExpected)

		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		// assign values to lastCheck.Metrics, lastCheckExpected and eventExpected
		// after declaration because need to call InitTriggerChecker first
		lastCheck.Metrics = map[string]*moira.MetricState{
			"Metric 1": {
				EventTimestamp: 0,
				Timestamp:      triggerChecker.CheckStarted - lastCheckDelta,
				State:          moira.OK,
				IsPending:      true,
				IsNoData:       true,
			},
		}

		lastCheckExpected.Metrics = lastCheck.Metrics
		lastCheckExpected.IsPending = true
		lastCheckExpected.Score = 100
		lastCheckExpected.EventTimestamp = triggerChecker.CheckStarted
		lastCheckExpected.Timestamp = triggerChecker.CheckStarted
		lastCheckExpected.Message = "Trigger has no metrics, check your target"
		lastCheckExpected.MaintenanceMetric = make(map[string]int64)
		lastCheckExpected.Metrics = map[string]*moira.MetricState{
			"Metric 1": {
				EventTimestamp: lastCheckExpected.EventTimestamp,
				// Timestamp has changed -- event has been emitted
				Timestamp: triggerChecker.CheckStarted,
				// so has State
				State:    moira.ERROR,
				IsNoData: true,
			},
		}

		eventExpected.Timestamp = triggerChecker.CheckStarted

		err = triggerChecker.Check()
		So(err, ShouldBeNil)
	})
}

func TestPendingIntervalThrottling(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	expression := "t1 > 0 ? OK : ERROR"
	metric := "trigger.metric"
	pattern := "trigger.pattern"

	triggerID := uuid.NewV4().String()
	trigger := &moira.Trigger{
		ID:              triggerID,
		TTL:             600,
		PendingInterval: 600,
		Expression:      &expression,
		Patterns:        []string{pattern},
		Targets:         []string{pattern},
	}

	now := time.Now().Unix() / 10 * 10 // truncate to 10
	metricValues := []*moira.MetricValue{
		{
			RetentionTimestamp: now - 50,
			Timestamp:          now - 47,
			Value:              0,
		},
		{
			RetentionTimestamp: now - 40,
			Timestamp:          now - 37,
			Value:              1,
		},
		{
			RetentionTimestamp: now - 30,
			Timestamp:          now - 27,
			Value:              0,
		},
		{
			RetentionTimestamp: now - 20,
			Timestamp:          now - 17,
			Value:              1,
		},
	}
	dataList := map[string][]*moira.MetricValue{
		metric: metricValues,
	}

	Convey("Several notifications during single pending interval", t, func() {
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		val1 := 1.0
		triggerChecker := TriggerChecker{
			TriggerID:    triggerID,
			Database:     dataBase,
			logger:       test_helpers.GetTestLogger(),
			silencer:     silencer.NewSilencer(dataBase, nil),
			CheckStarted: now,
			Until:        now,
			Config:       &Config{MetricsTTLSeconds: 0},
		}
		lastCheck := &moira.CheckData{
			Metrics: map[string]*moira.MetricState{
				metric: {
					EventTimestamp: 0,
					Timestamp:      now - 60,
					State:          moira.OK,
					Value:          &val1,
				},
			},
		}

		// event isn't emitted though metric changes its state all the time
		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(lastCheck, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)

		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(int64(10), nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, now-trigger.TTL-trigger.PendingInterval, now).Return(dataList, nil)
		dataBase.EXPECT().RemoveMetricsValues([]string{metric}, triggerChecker.Until-triggerChecker.Config.MetricsTTLSeconds)

		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		_, err = triggerChecker.handleTrigger()
		So(err, ShouldBeNil)
	})
}
