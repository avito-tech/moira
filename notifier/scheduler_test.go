package notifier

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/mock/moira-alert"
)

func TestThrottling(t *testing.T) {
	var trigger = moira.TriggerData{
		ID:         "triggerID-0000000000001",
		Name:       "test trigger",
		Targets:    []string{"test.target.5"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag"},
	}

	subID := "SubscriptionID-000000000000001"

	var event = moira.NotificationEvent{
		Metric:         "generate.event.1",
		State:          moira.OK,
		OldState:       moira.WARN,
		TriggerID:      trigger.ID,
		SubscriptionID: &subID,
	}

	now := time.Now()

	Convey("Test sendFail more than 0, and no throttling, should send message in one minute", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		notifierMetrics := metrics.NewNotifierMetrics()
		scheduler := NewScheduler(dataBase, notifierMetrics)

		expectedNext := now.Add(2 * time.Minute).Unix()
		next, throttling := scheduler.GetDeliveryInfo(now, event, false, 1)

		So(next.Unix(), ShouldEqual, expectedNext)
		So(throttling, ShouldBeFalse)
	})

	Convey("Test sendFail more than 0, and has throttling, should send message in one minute", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		notifierMetrics := metrics.NewNotifierMetrics()
		scheduler := NewScheduler(dataBase, notifierMetrics)

		expectedNext := now.Add(8 * time.Minute).Unix()
		next, throttling := scheduler.GetDeliveryInfo(now, event, true, 3)

		So(next.Unix(), ShouldEqual, expectedNext)
		So(throttling, ShouldBeTrue)
	})

	Convey("Test event state is TEST and no send fails, should return now notification time", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		notifierMetrics := metrics.NewNotifierMetrics()
		scheduler := NewScheduler(dataBase, notifierMetrics)

		subID := "SubscriptionID-000000000000001"
		testEvent := moira.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira.TEST,
			OldState:       moira.WARN,
			TriggerID:      trigger.ID,
			SubscriptionID: &subID,
		}
		expectedNext := now.Unix()

		next, throttling := scheduler.GetDeliveryInfo(now, testEvent, false, 0)
		So(next.Unix(), ShouldEqual, expectedNext)
		So(throttling, ShouldBeFalse)
	})

	Convey("Test no throttling and no subscription, should return now notification time", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		notifierMetrics := metrics.NewNotifierMetrics()
		scheduler := NewScheduler(dataBase, notifierMetrics)

		dataBase.EXPECT().GetTriggerThrottling(trigger.ID).Times(1).Return(time.Unix(0, 0), time.Unix(0, 0))
		dataBase.EXPECT().GetSubscription(*event.SubscriptionID).Times(1).Return(moira.SubscriptionData{}, fmt.Errorf("Error while read subscription"))
		expectedNext := now.Unix()
		next, throttling := scheduler.GetDeliveryInfo(now, event, false, 0)
		So(next.Unix(), ShouldEqual, expectedNext)
		So(throttling, ShouldBeFalse)
	})
}
