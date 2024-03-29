package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/satori/go.uuid"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/mock/moira-alert"
)

func TestUpdateTrigger(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

	Convey("Success update", t, func() {
		triggerModel := dto.TriggerModel{ID: uuid.NewV4().String()}
		trigger := triggerModel.ToMoiraTrigger()
		dataBase.EXPECT().GetTrigger(triggerModel.ID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(gomock.Any())
		dataBase.EXPECT().DeleteTriggerCheckLock(gomock.Any())
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(gomock.Any()).Return(&moira.CheckData{
			Metrics: make(map[string]*moira.MetricState),
		}, nil)
		dataBase.EXPECT().SaveTrigger(gomock.Any(), trigger).Return(nil)
		dataBase.EXPECT().SetTriggerLastCheck(gomock.Any(), gomock.Any()).Return(nil)
		resp, err := UpdateTrigger(dataBase, nil, &triggerModel, triggerModel.ID, make(map[string]bool))
		So(err, ShouldBeNil)
		So(resp.Message, ShouldResemble, "trigger updated")
	})

	Convey("Trigger does not exists", t, func() {
		trigger := dto.TriggerModel{ID: uuid.NewV4().String()}
		dataBase.EXPECT().GetTrigger(trigger.ID).Return(nil, database.ErrNil)
		resp, err := UpdateTrigger(dataBase, nil, &trigger, trigger.ID, make(map[string]bool))
		So(err, ShouldResemble, api.ErrorNotFound(fmt.Sprintf("Trigger with ID = '%s' does not exists", trigger.ID)))
		So(resp, ShouldBeNil)
	})

	Convey("Get trigger error", t, func() {
		trigger := dto.TriggerModel{ID: uuid.NewV4().String()}
		expected := fmt.Errorf("Soo bad trigger")
		dataBase.EXPECT().GetTrigger(trigger.ID).Return(nil, expected)
		resp, err := UpdateTrigger(dataBase, nil, &trigger, trigger.ID, make(map[string]bool))
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(resp, ShouldBeNil)
	})
}

func TestSaveTrigger(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()
	trigger := moira.Trigger{ID: triggerID}
	lastCheck := &moira.CheckData{
		Metrics: map[string]*moira.MetricState{
			"super.metric1": {},
			"super.metric2": {},
		},
	}
	emptyLastCheck := &moira.CheckData{
		Metrics: make(map[string]*moira.MetricState),
	}

	Convey("No timeSeries", t, func() {
		Convey("No last check", func() {
			dataBase.EXPECT().AcquireTriggerCheckLock(triggerID)
			dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
			dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerID).Return(&moira.CheckData{
				Metrics: make(map[string]*moira.MetricState),
			}, nil)
			dataBase.EXPECT().SaveTrigger(triggerID, &trigger).Return(nil)
			dataBase.EXPECT().SetTriggerLastCheck(triggerID, gomock.Any()).Return(nil)
			resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, make(map[string]bool))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &dto.SaveTriggerResponse{ID: triggerID, Message: "trigger updated"})
		})
		Convey("Has last check", func() {
			actualLastCheck := lastCheck
			dataBase.EXPECT().AcquireTriggerCheckLock(triggerID)
			dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
			dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerID).Return(actualLastCheck, nil)
			dataBase.EXPECT().SaveTrigger(triggerID, &trigger).Return(nil)
			dataBase.EXPECT().SetTriggerLastCheck(triggerID, actualLastCheck).Return(nil)
			resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, make(map[string]bool))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &dto.SaveTriggerResponse{ID: triggerID, Message: "trigger updated"})
			So(actualLastCheck, ShouldResemble, emptyLastCheck)
		})
	})

	Convey("Has timeSeries", t, func() {
		actualLastCheck := lastCheck
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerID).Return(&moira.CheckData{
			Metrics: make(map[string]*moira.MetricState),
		}, nil)
		dataBase.EXPECT().SaveTrigger(triggerID, &trigger).Return(nil)
		dataBase.EXPECT().SetTriggerLastCheck(triggerID, gomock.Any()).Return(nil)
		resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, map[string]bool{"super.metric1": true, "super.metric2": true})
		So(err, ShouldBeNil)
		So(resp, ShouldResemble, &dto.SaveTriggerResponse{ID: triggerID, Message: "trigger updated"})
		So(actualLastCheck, ShouldResemble, lastCheck)
	})

	Convey("Errors", t, func() {
		Convey("AcquireTriggerCheckLock error", func() {
			expected := fmt.Errorf("AcquireTriggerCheckLock error")
			dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(expected)
			resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, make(map[string]bool))
			So(err, ShouldResemble, api.ErrorInternalServer(expected))
			So(resp, ShouldBeNil)
		})

		Convey("GetTriggerLastCheck error", func() {
			expected := fmt.Errorf("GetTriggerLastCheck error")
			dataBase.EXPECT().AcquireTriggerCheckLock(triggerID)
			dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
			dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerID).Return(nil, expected)
			resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, make(map[string]bool))
			So(err, ShouldResemble, api.ErrorInternalServer(expected))
			So(resp, ShouldBeNil)
		})

		Convey("SetTriggerLastCheck error", func() {
			expected := fmt.Errorf("SetTriggerLastCheck error")
			dataBase.EXPECT().AcquireTriggerCheckLock(triggerID)
			dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
			dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerID).Return(&moira.CheckData{
				Metrics: make(map[string]*moira.MetricState),
			}, nil)
			dataBase.EXPECT().SaveTrigger(triggerID, &trigger)
			dataBase.EXPECT().SetTriggerLastCheck(triggerID, gomock.Any()).Return(expected)
			resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, make(map[string]bool))
			So(err, ShouldResemble, api.ErrorInternalServer(expected))
			So(resp, ShouldBeNil)
		})

		Convey("saveTrigger error", func() {
			expected := fmt.Errorf("saveTrigger error")
			dataBase.EXPECT().AcquireTriggerCheckLock(triggerID)
			dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
			dataBase.EXPECT().GetOrCreateTriggerLastCheck(triggerID).Return(&moira.CheckData{
				Metrics: make(map[string]*moira.MetricState),
			}, nil)
			dataBase.EXPECT().SaveTrigger(triggerID, &trigger).Return(expected)
			resp, err := saveTrigger(dataBase, nil, &trigger, triggerID, make(map[string]bool))
			So(err, ShouldResemble, api.ErrorInternalServer(expected))
			So(resp, ShouldBeNil)
		})
	})
}

func TestGetTrigger(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()
	triggerModel := dto.TriggerModel{ID: triggerID}
	trigger := triggerModel.ToMoiraTrigger()
	begging := time.Unix(0, 0)
	now := time.Now()
	tomorrow := now.Add(time.Hour * 24)
	yesterday := now.Add(-time.Hour * 24)

	Convey("Has trigger no throttling", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerThrottling(triggerID).Return(begging, begging)
		dataBase.EXPECT().TriggerHasPendingEscalations(triggerID, false).Return(false, nil)
		dataBase.EXPECT().GetTriggers(trigger.Parents).Return(nil, nil)
		actual, err := GetTrigger(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.Trigger{
			ParentTriggers: make([]*dto.Trigger, 0),
			TriggerModel:   triggerModel,
			Throttling:     0,
		})
	})

	Convey("Has trigger has throttling", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerThrottling(triggerID).Return(tomorrow, begging)
		dataBase.EXPECT().TriggerHasPendingEscalations(triggerID, false).Return(false, nil)
		dataBase.EXPECT().GetTriggers(trigger.Parents).Return(nil, nil)
		actual, err := GetTrigger(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.Trigger{
			ParentTriggers: make([]*dto.Trigger, 0),
			TriggerModel:   triggerModel,
			Throttling:     tomorrow.Unix(),
		})
	})

	Convey("Has trigger has old throttling", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerThrottling(triggerID).Return(yesterday, begging)
		dataBase.EXPECT().TriggerHasPendingEscalations(triggerID, false).Return(false, nil)
		dataBase.EXPECT().GetTriggers(trigger.Parents).Return(nil, nil)
		actual, err := GetTrigger(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.Trigger{
			ParentTriggers: make([]*dto.Trigger, 0),
			TriggerModel:   triggerModel,
			Throttling:     0,
		})
	})

	Convey("GetTrigger error", t, func() {
		expected := fmt.Errorf("GetTrigger error")
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{}, expected)
		actual, err := GetTrigger(dataBase, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(actual, ShouldBeNil)
	})

	Convey("No trigger", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{}, database.ErrNil)
		actual, err := GetTrigger(dataBase, triggerID)
		So(err, ShouldResemble, api.ErrorNotFound("Trigger not found"))
		So(actual, ShouldBeNil)
	})
}

func TestRemoveTrigger(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()

	Convey("Success", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(nil, nil)
		dataBase.EXPECT().RemoveTrigger(triggerID).Return(nil)
		dataBase.EXPECT().RemoveTriggerLastCheck(triggerID).Return(nil)
		err := RemoveTrigger(dataBase, triggerID)
		So(err, ShouldBeNil)
	})

	Convey("Error remove trigger", t, func() {
		expected := fmt.Errorf("Oooops! Error delete")
		dataBase.EXPECT().GetTrigger(triggerID).Return(nil, nil)
		dataBase.EXPECT().RemoveTrigger(triggerID).Return(expected)
		err := RemoveTrigger(dataBase, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})

	Convey("Error remove last check", t, func() {
		expected := fmt.Errorf("Oooops! Error delete")
		dataBase.EXPECT().GetTrigger(triggerID).Return(nil, nil)
		dataBase.EXPECT().RemoveTrigger(triggerID).Return(nil)
		dataBase.EXPECT().RemoveTriggerLastCheck(triggerID).Return(expected)
		err := RemoveTrigger(dataBase, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})
}

func TestGetTriggerThrottling(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()
	begging := time.Unix(0, 0)
	now := time.Now()
	tomorrow := now.Add(time.Hour * 24)
	yesterday := now.Add(-time.Hour * 24)

	Convey("no throttling", t, func() {
		dataBase.EXPECT().GetTriggerThrottling(triggerID).Return(begging, begging)
		actual, err := GetTriggerThrottling(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.ThrottlingResponse{Throttling: 0})
	})

	Convey("has throttling", t, func() {
		dataBase.EXPECT().GetTriggerThrottling(triggerID).Return(tomorrow, begging)
		actual, err := GetTriggerThrottling(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.ThrottlingResponse{Throttling: tomorrow.Unix()})
	})

	Convey("has old throttling", t, func() {
		dataBase.EXPECT().GetTriggerThrottling(triggerID).Return(yesterday, begging)
		actual, err := GetTriggerThrottling(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.ThrottlingResponse{Throttling: 0})
	})
}

func TestGetTriggerLastCheck(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()
	lastCheck := &moira.CheckData{}

	Convey("Success", t, func() {
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(lastCheck, nil)
		dataBase.EXPECT().GetMaintenanceTrigger(triggerID).Return(moira.NewMaintenance(), nil)

		check, err := GetTriggerLastCheck(dataBase, triggerID)
		So(err, ShouldBeNil)
		So(check, ShouldResemble, &dto.TriggerCheck{
			TriggerID: triggerID,
			CheckData: lastCheck,
		})
	})

	Convey("Error", t, func() {
		expected := fmt.Errorf("Oooops! Error get")
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(&moira.CheckData{}, expected)
		check, err := GetTriggerLastCheck(dataBase, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(check, ShouldBeNil)
	})
}

func TestDeleteTriggerThrottling(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()

	Convey("Success", t, func() {
		dataBase.EXPECT().DeleteTriggerThrottling(triggerID).Return(nil)
		var total int64
		var to int64 = -1
		dataBase.EXPECT().GetNotifications(total, to).Return(make([]*moira.ScheduledNotification, 0), total, nil)
		dataBase.EXPECT().AddNotifications(make([]*moira.ScheduledNotification, 0), gomock.Any()).Return(nil)
		err := DeleteTriggerThrottling(dataBase, triggerID)
		So(err, ShouldBeNil)
	})

	Convey("Error", t, func() {
		expected := fmt.Errorf("Oooops! Error delete")
		dataBase.EXPECT().DeleteTriggerThrottling(triggerID).Return(expected)
		err := DeleteTriggerThrottling(dataBase, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})
}

func TestDeleteTriggerMetric(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()
	trigger := &moira.Trigger{ID: triggerID}
	lastCheck := moira.CheckData{
		Metrics: map[string]*moira.MetricState{
			"super.metric1": {},
		},
	}
	emptyLastCheck := &moira.CheckData{
		Metrics: make(map[string]*moira.MetricState),
	}

	Convey("Success delete from last check", t, func() {
		expectedLastCheck := &lastCheck
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(nil)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(expectedLastCheck, nil)
		dataBase.EXPECT().RemovePatternsMetrics(trigger.Patterns).Return(nil)
		dataBase.EXPECT().SetTriggerLastCheck(triggerID, expectedLastCheck)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldBeNil)
		So(expectedLastCheck, ShouldResemble, emptyLastCheck)
	})

	Convey("Success delete nothing to delete", t, func() {
		expectedLastCheck := emptyLastCheck
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(nil)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(expectedLastCheck, nil)
		dataBase.EXPECT().RemovePatternsMetrics(trigger.Patterns).Return(nil)
		dataBase.EXPECT().SetTriggerLastCheck(triggerID, expectedLastCheck)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldBeNil)
		So(expectedLastCheck, ShouldResemble, emptyLastCheck)
	})

	Convey("No trigger", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{}, database.ErrNil)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInvalidRequest(fmt.Errorf("Trigger not found")))
	})

	Convey("No last check", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(nil)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(&moira.CheckData{}, database.ErrNil)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInvalidRequest(fmt.Errorf("Trigger check not found")))
	})

	Convey("Get trigger error", t, func() {
		expected := fmt.Errorf("Get trigger error")
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{}, expected)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})

	Convey("AcquireTriggerCheckLock error", t, func() {
		expected := fmt.Errorf("Acquire error")
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(expected)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})

	Convey("GetTriggerLastCheck error", t, func() {
		expected := fmt.Errorf("Last check error")
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(nil)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(&moira.CheckData{}, expected)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})

	Convey("RemovePatternsMetrics error", t, func() {
		expected := fmt.Errorf("RemovePatternsMetrics err")
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(nil)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(&lastCheck, nil)
		dataBase.EXPECT().RemovePatternsMetrics(trigger.Patterns).Return(expected)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})

	Convey("SetTriggerLastCheck error", t, func() {
		expected := fmt.Errorf("RemovePatternsMetrics err")
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().AcquireTriggerCheckLock(triggerID).Return(nil)
		dataBase.EXPECT().DeleteTriggerCheckLock(triggerID)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(&lastCheck, nil)
		dataBase.EXPECT().RemovePatternsMetrics(trigger.Patterns).Return(nil)
		dataBase.EXPECT().SetTriggerLastCheck(triggerID, &lastCheck).Return(expected)
		err := DeleteTriggerMetric(dataBase, "super.metric1", triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})
}

func TestSetMetricsMaintenance(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()

	maintenance := moira.NewMaintenance()
	update := make(map[string]int64)

	Convey("Success", t, func() {
		dataBase.EXPECT().AcquireTriggerMaintenanceLock(triggerID)
		dataBase.EXPECT().GetMaintenanceTrigger(triggerID).Return(maintenance, nil)
		dataBase.EXPECT().SetMaintenanceTrigger(triggerID, maintenance).Return(nil)
		dataBase.EXPECT().DeleteTriggerMaintenanceLock(triggerID)

		err := SetMetricsMaintenance(dataBase, triggerID, update)
		So(err, ShouldBeNil)
	})

	Convey("Error", t, func() {
		expected := fmt.Errorf("Oooops! Error set")
		dataBase.EXPECT().AcquireTriggerMaintenanceLock(triggerID)
		dataBase.EXPECT().GetMaintenanceTrigger(triggerID).Return(maintenance, nil)
		dataBase.EXPECT().SetMaintenanceTrigger(triggerID, maintenance).Return(expected)
		dataBase.EXPECT().DeleteTriggerMaintenanceLock(triggerID)

		err := SetMetricsMaintenance(dataBase, triggerID, update)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})
}

func TestGetTriggerMetrics(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	triggerID := uuid.NewV4().String()
	pattern := "super.puper.pattern"
	metric := "super.puper.metric"
	dataList := map[string][]*moira.MetricValue{
		metric: {
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
		},
	}

	var from int64 = 17
	var until int64 = 67
	var retention int64 = 10

	Convey("Has metrics", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{ID: triggerID, Targets: []string{pattern}}, nil)
		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, from, until).Return(dataList, nil)
		triggerMetrics, err := GetTriggerMetrics(dataBase, from, until, triggerID)
		So(err, ShouldBeNil)
		So(triggerMetrics, ShouldResemble, dto.TriggerMetrics(map[string][]moira.MetricValue{metric: {{Value: 0, Timestamp: 17}, {Value: 1, Timestamp: 27}, {Value: 2, Timestamp: 37}, {Value: 3, Timestamp: 47}, {Value: 4, Timestamp: 57}}}))
	})

	Convey("GetTrigger error", t, func() {
		expected := fmt.Errorf("Get trigger error")
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{}, expected)
		triggerMetrics, err := GetTriggerMetrics(dataBase, from, until, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(triggerMetrics, ShouldBeNil)
	})

	Convey("No trigger", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{}, database.ErrNil)
		triggerMetrics, err := GetTriggerMetrics(dataBase, from, until, triggerID)
		So(err, ShouldResemble, api.ErrorInvalidRequest(fmt.Errorf("Trigger not found")))
		So(triggerMetrics, ShouldBeNil)
	})

	Convey("GetMetricsValues error", t, func() {
		expected := fmt.Errorf("GetMetricsValues error")
		dataBase.EXPECT().GetTrigger(triggerID).Return(&moira.Trigger{ID: triggerID, Targets: []string{pattern}}, nil)
		dataBase.EXPECT().GetPatternMetrics(pattern).Return([]string{metric}, nil)
		dataBase.EXPECT().GetMetricRetention(metric).Return(retention, nil)
		dataBase.EXPECT().GetMetricsValues([]string{metric}, from, until).Return(nil, expected)
		triggerMetrics, err := GetTriggerMetrics(dataBase, from, until, triggerID)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(triggerMetrics, ShouldBeNil)
	})

}
