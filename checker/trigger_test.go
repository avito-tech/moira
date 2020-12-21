package checker

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/silencer"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestInitTriggerChecker(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerChecker := TriggerChecker{
		TriggerID: "342a28dc-7a27-4f3f-9a45-a010cf988aa7",
		Database:  dataBase,
		logger:    test_helpers.GetTestLogger(),
		silencer:  silencer.NewSilencer(dataBase, nil),
	}

	Convey("Test errors", t, func() {
		Convey("Get trigger error", func() {
			getTriggerError := fmt.Errorf("Oppps! Can't read trigger")
			dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&moira.Trigger{}, getTriggerError)
			err := triggerChecker.InitTriggerChecker()
			So(err, ShouldBeError)
			So(err, ShouldResemble, getTriggerError)
		})

		Convey("No trigger error", func() {
			dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&moira.Trigger{}, database.ErrNil)
			err := triggerChecker.InitTriggerChecker()
			So(err, ShouldBeError)
			So(err, ShouldResemble, ErrTriggerNotExists)
		})

		Convey("Get lastCheck error", func() {
			readLastCheckError := fmt.Errorf("Oppps! Can't read last check")
			dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&moira.Trigger{}, nil)
			dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(&moira.CheckData{}, readLastCheckError)
			err := triggerChecker.InitTriggerChecker()
			So(err, ShouldBeError)
			So(err, ShouldResemble, readLastCheckError)
		})
	})

	var warnWalue float64 = 10000
	var errorWalue float64 = 10000
	var ttl int64 = 900
	var value float64
	ttlStateOk := moira.OK
	ttlStateNoData := moira.NODATA

	trigger := moira.Trigger{
		ID:         "d39b8510-b2f4-448c-b881-824658c58128",
		Name:       "Time",
		Targets:    []string{"aliasByNode(Metric.*.time, 1)"},
		WarnValue:  &warnWalue,
		ErrorValue: &errorWalue,
		Tags:       []string{"tag1", "tag2"},
		TTLState:   &ttlStateOk,
		Patterns:   []string{"Egais.elasticsearch.*.*.jvm.gc.collection.time"},
		TTL:        ttl,
	}

	lastCheck := moira.CheckData{
		Timestamp:         1502694487,
		State:             moira.OK,
		Score:             0,
		MaintenanceMetric: map[string]int64{},
		Metrics: map[string]*moira.MetricState{
			"1": {
				Timestamp:      1502694427,
				State:          moira.OK,
				Suppressed:     false,
				Value:          &value,
				EventTimestamp: 1501680428,
			},
			"2": {
				Timestamp:      1502694427,
				State:          moira.OK,
				Suppressed:     false,
				Value:          &value,
				EventTimestamp: 1501679827,
			},
			"3": {
				Timestamp:      1502694427,
				State:          moira.OK,
				Suppressed:     false,
				Value:          &value,
				EventTimestamp: 1501679887,
			},
		},
	}

	triggerChecker = TriggerChecker{
		TriggerID: trigger.ID,
		Database:  dataBase,
		logger:    test_helpers.GetTestLogger(),
		silencer:  silencer.NewSilencer(dataBase, nil),
	}

	Convey("Test trigger checker with lastCheck", t, func() {
		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(&lastCheck, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)
		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		expectedTriggerChecker := triggerChecker
		expectedTriggerChecker.trigger = &trigger
		expectedTriggerChecker.ttl = trigger.TTL
		expectedTriggerChecker.ttlState = *trigger.TTLState
		expectedTriggerChecker.lastCheck = &lastCheck
		expectedTriggerChecker.lastCheck.Timestamp = triggerChecker.lastCheck.Timestamp
		expectedTriggerChecker.From = lastCheck.Timestamp

		So(triggerChecker, ShouldResemble, expectedTriggerChecker)
	})

	Convey("Test trigger checker without lastCheck", t, func() {
		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(&moira.CheckData{
			MaintenanceMetric: make(map[string]int64),
			Metrics:           make(map[string]*moira.MetricState),
			State:             moira.NODATA,
		}, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)
		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		expectedTriggerChecker := triggerChecker
		expectedTriggerChecker.trigger = &trigger
		expectedTriggerChecker.ttl = trigger.TTL
		expectedTriggerChecker.ttlState = *trigger.TTLState
		expectedTriggerChecker.lastCheck = &moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics:           make(map[string]*moira.MetricState),
			State:             moira.NODATA,
			Timestamp:         expectedTriggerChecker.Until - ttl,
		}
		expectedTriggerChecker.From = expectedTriggerChecker.Until - ttl
		So(triggerChecker, ShouldResemble, expectedTriggerChecker)
	})

	trigger.TTL = 0
	trigger.TTLState = nil

	Convey("Test trigger checker without lastCheck and ttl", t, func() {
		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(&moira.CheckData{
			MaintenanceMetric: make(map[string]int64),
			Metrics:           make(map[string]*moira.MetricState),
			State:             moira.NODATA,
		}, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)
		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		expectedTriggerChecker := triggerChecker
		expectedTriggerChecker.trigger = &trigger
		expectedTriggerChecker.ttl = 0
		expectedTriggerChecker.ttlState = ttlStateNoData
		expectedTriggerChecker.lastCheck = &moira.CheckData{
			MaintenanceMetric: map[string]int64{},
			Metrics:           make(map[string]*moira.MetricState),
			State:             moira.NODATA,
			Timestamp:         expectedTriggerChecker.Until - 600,
		}
		expectedTriggerChecker.From = expectedTriggerChecker.Until - 600
		So(triggerChecker, ShouldResemble, expectedTriggerChecker)
	})

	Convey("Test trigger checker with lastCheck and without ttl", t, func() {
		dataBase.EXPECT().GetTrigger(triggerChecker.TriggerID).Return(&trigger, nil)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerChecker.TriggerID).Return(&lastCheck, nil)
		dataBase.EXPECT().GetTriggerForcedNotifications(triggerChecker.TriggerID).Return(nil, nil)
		err := triggerChecker.InitTriggerChecker()
		So(err, ShouldBeNil)

		expectedTriggerChecker := triggerChecker
		expectedTriggerChecker.trigger = &trigger
		expectedTriggerChecker.ttl = 0
		expectedTriggerChecker.ttlState = ttlStateNoData
		expectedTriggerChecker.lastCheck = &lastCheck
		expectedTriggerChecker.From = lastCheck.Timestamp
		So(triggerChecker, ShouldResemble, expectedTriggerChecker)
	})
}
