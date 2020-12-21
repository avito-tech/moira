package notifier

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/mock/scheduler"
	"go.avito.ru/DO/moira/test-helpers"
)

var (
	mockCtrl  *gomock.Controller
	sender    *mock_moira_alert.MockSender
	notif     *StandardNotifier
	scheduler *mock_scheduler.MockScheduler
	dataBase  *mock_moira_alert.MockDatabase
)

func TestUnknownContactType(t *testing.T) {
	configureNotifier(t)
	defer afterTest()

	eventsData := []moira.NotificationEvent{event}
	pkg := NotificationPackage{
		Events: eventsData,
		Contact: moira.ContactData{
			Type: "unknown contact",
		},
	}
	notification := moira.ScheduledNotification{}

	scheduler.EXPECT().CalculateBackoff(pkg.FailCount + 1).Return(1 * time.Minute)
	scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, pkg.Throttled, pkg.FailCount+1).Return(time.Now(), false)
	scheduler.EXPECT().ScheduleNotification(gomock.Any(), pkg.Throttled, event, pkg.Trigger, pkg.Contact, pkg.FailCount+1, pkg.NeedAck).Return(&notification)
	dataBase.EXPECT().AddNotification(&notification).Return(nil)

	wg := sync.WaitGroup{}
	notif.Send(&pkg, &wg)
	wg.Wait()
}

func TestFailSendEvent(t *testing.T) {
	configureNotifier(t)
	defer afterTest()

	var eventsData moira.NotificationEvents = []moira.NotificationEvent{event}

	pkg := NotificationPackage{
		Events: eventsData,
		Contact: moira.ContactData{
			Type: "test",
		},
	}
	notification := moira.ScheduledNotification{}

	sender.EXPECT().SendEvents(eventsData, pkg.Contact, pkg.Trigger, pkg.Throttled, pkg.NeedAck).Return(fmt.Errorf("Can't send"))
	scheduler.EXPECT().CalculateBackoff(pkg.FailCount + 1).Return(1 * time.Minute)
	scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, pkg.Throttled, pkg.FailCount+1).Return(time.Now(), false)
	scheduler.EXPECT().ScheduleNotification(gomock.Any(), pkg.Throttled, event, pkg.Trigger, pkg.Contact, pkg.FailCount+1, pkg.NeedAck).Return(&notification)
	dataBase.EXPECT().AddNotification(&notification).Return(nil)

	var wg sync.WaitGroup
	notif.Send(&pkg, &wg)
	wg.Wait()
	time.Sleep(time.Second * 2)
}

func configureNotifier(t *testing.T) {
	test_helpers.InitTestLogging()
	notifierMetrics := metrics.NewNotifierMetrics()

	var location, _ = time.LoadLocation("UTC")
	config := Config{
		SendingTimeout:   time.Millisecond * 10,
		ResendingTimeout: time.Hour * 24,
		Location:         location,
	}
	mockCtrl = gomock.NewController(t)

	dataBase = mock_moira_alert.NewMockDatabase(mockCtrl)
	test_helpers.InitTestLogging()

	scheduler = mock_scheduler.NewMockScheduler(mockCtrl)
	sender = mock_moira_alert.NewMockSender(mockCtrl)

	notif = NewNotifier(dataBase, config, notifierMetrics)
	notif.scheduler = scheduler
	senderSettings := map[string]string{
		"type": "test",
	}

	sender.EXPECT().Init(senderSettings, location).Return(nil)

	notif.RegisterSender(senderSettings, sender)

	Convey("Should return one sender", t, func() {
		So(notif.GetSenders(), ShouldResemble, map[string]bool{"test": true})
	})
}

func afterTest() {
	mockCtrl.Finish()
	notif.StopSenders()
}

var subID = "SubscriptionID-000000000000001"

var event = moira.NotificationEvent{
	Metric:         "generate.event.1",
	State:          moira.OK,
	OldState:       moira.WARN,
	TriggerID:      "triggerID-0000000000001",
	SubscriptionID: &subID,
}
