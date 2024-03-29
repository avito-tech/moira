package events

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/satori/go.uuid"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/mock/scheduler"
	"go.avito.ru/DO/moira/notifier"
	"go.avito.ru/DO/moira/test-helpers"
)

var (
	logger          moira.Logger
	notifierMetrics *metrics.NotifierMetrics
)

func init() {
	test_helpers.InitTestLogging()

	logger = test_helpers.GetTestLogger()
	notifierMetrics = metrics.NewNotifierMetrics()
}

func TestEvent(t *testing.T) {
	Convey("When event is TEST and subscription is disabled, should add new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),
		}
		event := moira.NotificationEvent{
			State:          moira.TEST,
			SubscriptionID: &subscription.ID,
		}
		dataBase.EXPECT().GetSubscription(*event.SubscriptionID).Times(1).Return(subscription, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		notification := moira.ScheduledNotification{
			Event: moira.NotificationEvent{
				TriggerID:      event.TriggerID,
				State:          event.State,
				OldState:       event.OldState,
				Metric:         event.Metric,
				SubscriptionID: event.SubscriptionID,
			},
			SendFail:  0,
			Timestamp: time.Now().Unix(),
			Throttled: false,
			Contact:   contact,
		}
		dataBase.EXPECT().AddNotification(&notification)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})

	Convey("When event is TEST and has contactID", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		subID := "testSubscription"
		event := moira.NotificationEvent{
			State:     moira.TEST,
			OldState:  moira.TEST,
			Metric:    "test.metric",
			ContactID: contact.ID,
		}
		dataBase.EXPECT().GetContact(event.ContactID).Times(1).Return(contact, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		now := time.Now()
		notification := moira.ScheduledNotification{
			Event: moira.NotificationEvent{
				TriggerID:      "",
				State:          event.State,
				OldState:       event.OldState,
				Metric:         event.Metric,
				SubscriptionID: &subID,
			},
			SendFail:  0,
			Timestamp: now.Unix(),
			Throttled: false,
			Contact:   contact,
		}
		event2 := event
		event2.SubscriptionID = &subID
		scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, false, 0).Times(1)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), false, event2, moira.TriggerData{}, contact, 0, false).Return(&notification)
		dataBase.EXPECT().AddNotification(&notification)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestNoSubscription(t *testing.T) {
	Convey("When no subscription by event tags, should not call AddNotification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),

			Fetcher: func() (events moira.NotificationEvents, err error) {
				event, err := dataBase.FetchNotificationEvent(false)
				return moira.NotificationEvents{event}, err
			},
		}

		event := moira.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira.OK,
			OldState:  moira.WARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(append(triggerData.Tags, event.GetEventTags()...)).Times(1).Return(make([]*moira.SubscriptionData, 0), nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestDisabledNotification(t *testing.T) {
	Convey("When subscription event tags is disabled, should not call AddNotification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),
		}

		event := moira.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira.OK,
			OldState:  moira.WARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{&disabledSubscription}, nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestExtraTags(t *testing.T) {
	Convey("When trigger has not all subscription tags, should not call AddNotification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),
		}

		event := moira.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira.OK,
			OldState:  moira.WARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{&multipleTagsSubscription}, nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestAddNotification(t *testing.T) {
	Convey("When good subscription, should add new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		event := moira.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira.OK,
			OldState:       moira.WARN,
			TriggerID:      triggerData.ID,
			SubscriptionID: &subscription.ID,
		}
		emptyNotification := moira.ScheduledNotification{}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{&subscription}, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, false, 0).Times(1)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), false, event, triggerData, contact, 0, false).Times(1).Return(&emptyNotification)
		dataBase.EXPECT().AddNotification(&emptyNotification).Times(1).Return(nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestAddOneNotificationByTwoSubscriptionsWithSame(t *testing.T) {
	Convey("When good subscription and create 2 same scheduled notifications, should add one new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		event := moira.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira.OK,
			OldState:       moira.WARN,
			TriggerID:      triggerData.ID,
			SubscriptionID: &subscription.ID,
		}
		event2 := event
		event2.SubscriptionID = &subscription4.ID

		notification2 := moira.ScheduledNotification{}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{&subscription, &subscription4}, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(2).Return(contact, nil)

		scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, false, 0).Times(1)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), false, event, triggerData, contact, 0, false).Times(1).Return(&notification2)
		scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, false, 0).Times(1)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), false, event2, triggerData, contact, 0, false).Times(1).Return(&notification2)

		dataBase.EXPECT().AddNotification(&notification2).Times(1).Return(nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

// TODO :( reorder expect
//func TestFailReadContact(t *testing.T) {
//	Convey("When read contact returns error, should not call AddNotification and not crashed", t, func() {
//		mockCtrl := gomock.NewController(t)
//		defer mockCtrl.Finish()
//		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
//		logger := mock_moira_alert.NewMockLogger(mockCtrl)
//		worker := FetchEventsWorker{
//			Database:  dataBase,
//			Logger:    logger,
//			Metrics:   metrics2,
//			Scheduler: notifier.NewScheduler(dataBase, logger, metrics2),
//		}
//
//		event := moira.NotificationEvent{
//			Metric:    "generate.event.1",
//			State:     "OK",
//			OldState:  "WARN",
//			TriggerID: triggerData.ID,
//		}
//
//		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
//		tags := append(triggerData.Tags, event.GetEventTags()...)
//		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{&subscription}, nil)
//		getContactError := fmt.Errorf("Can not get contact")
//		dataBase.EXPECT().GetTriggerThrottling(event.TriggerID).Times(1).Return(time.Time{}, time.Time{})
//		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(moira.ContactData{}, getContactError)
//		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira.UseFloat64(event.Value), event.OldState, event.State)
//		logger.EXPECT().Debugf("Getting subscriptions for tags %v", tags)
//		logger.EXPECT().Debugf("Processing contact ids %v for subscription %s", subscription.Contacts, subscription.ID)
//		logger.EXPECT().Warningf("Failed to get contact: %s, skip handling it, error: %v", contact.ID, getContactError)
//
//		err := worker.processEvent(event)
//		So(err, ShouldBeEmpty)
//	})
//}

func TestEmptySubscriptions(t *testing.T) {
	Convey("When subscription is empty value object", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),
		}

		event := moira.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira.OK,
			OldState:  moira.WARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{{ThrottlingEnabled: true}}, nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})

	Convey("When subscription is nil", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),
		}

		event := moira.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira.OK,
			OldState:  moira.WARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{nil}, nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestGetNotificationSubscriptions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	worker := FetchEventsWorker{
		Database:  dataBase,
		Logger:    logger,
		Metrics:   notifierMetrics,
		Scheduler: notifier.NewScheduler(dataBase, notifierMetrics),
	}

	Convey("Error GetSubscription", t, func() {
		event := moira.NotificationEvent{
			State:          moira.TEST,
			SubscriptionID: &subscription.ID,
		}
		err := fmt.Errorf("Oppps")
		dataBase.EXPECT().GetSubscription(*event.SubscriptionID).Return(moira.SubscriptionData{}, err)
		sub, expected := worker.getNotificationSubscriptions(event)
		So(sub, ShouldBeNil)
		So(expected, ShouldResemble, fmt.Errorf("Error while read subscription %s: %s", *event.SubscriptionID, err.Error()))
	})

	Convey("Error GetContact", t, func() {
		event := moira.NotificationEvent{
			State:     moira.TEST,
			ContactID: "1233",
		}
		err := fmt.Errorf("Oppps")
		dataBase.EXPECT().GetContact(event.ContactID).Return(moira.ContactData{}, err)
		sub, expected := worker.getNotificationSubscriptions(event)
		So(sub, ShouldBeNil)
		So(expected, ShouldResemble, fmt.Errorf("Error while read contact %s: %s", event.ContactID, err.Error()))
	})

}

func TestGoRoutine(t *testing.T) {
	Convey("When good subscription, should add new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)

		worker := &FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,

			Fetcher: func() (events moira.NotificationEvents, err error) {
				event, err := dataBase.FetchNotificationEvent(false)
				return moira.NotificationEvents{event}, err
			},
		}

		event := moira.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira.OK,
			OldState:       moira.WARN,
			TriggerID:      triggerData.ID,
			SubscriptionID: &subscription.ID,
		}
		emptyNotification := moira.ScheduledNotification{}
		shutdown := make(chan bool)

		dataBase.EXPECT().FetchNotificationEvent(false).Return(moira.NotificationEvent{}, fmt.Errorf("3433434")).Do(func(f ...interface{}) {
			dataBase.EXPECT().FetchNotificationEvent(false).Return(event, nil).Do(func(f ...interface{}) {
				dataBase.EXPECT().FetchNotificationEvent(false).AnyTimes().Return(moira.NotificationEvent{}, database.ErrNil)
			})
		})
		dataBase.EXPECT().GetTrigger(event.TriggerID).Times(1).Return(trigger, nil)
		tags := append(triggerData.Tags, event.GetEventTags()...)
		dataBase.EXPECT().GetTagsSubscriptions(tags).Times(1).Return([]*moira.SubscriptionData{&subscription}, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		scheduler.EXPECT().GetDeliveryInfo(gomock.Any(), event, false, 0).Times(1)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), false, event, triggerData, contact, 0, false).Times(1).Return(&emptyNotification)
		dataBase.EXPECT().AddNotification(&emptyNotification).Times(1).Return(nil).Do(func(f ...interface{}) { close(shutdown) })

		worker.Start()
		waitTestEnd(shutdown, worker)
	})
}

func waitTestEnd(shutdown chan bool, worker *FetchEventsWorker) {
	select {
	case <-shutdown:
		_ = worker.Stop()
		break
	case <-time.After(time.Second * 10):
		close(shutdown)
		break
	}
}

var warnValue float64 = 10
var errorValue float64 = 20

var trigger = &moira.Trigger{
	ID:         uuid.NewV4().String(),
	Name:       "test trigger",
	Targets:    []string{"test.target.5"},
	WarnValue:  &warnValue,
	ErrorValue: &errorValue,
	Tags:       []string{"test-tag"},
}

var triggerData = moira.TriggerData{
	ID:         trigger.ID,
	Name:       "test trigger",
	Targets:    []string{"test.target.5"},
	WarnValue:  warnValue,
	ErrorValue: errorValue,
	Tags:       []string{"test-tag"},
}

var contact = moira.ContactData{
	ID:    "ContactID-000000000000001",
	Type:  "email",
	Value: "mail1@example.com",
}

var subscription = moira.SubscriptionData{
	ID:                "subscriptionID-00000000000001",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var subscription4 = moira.SubscriptionData{
	ID:                "subscriptionID-00000000000004",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var disabledSubscription = moira.SubscriptionData{
	ID:                "subscriptionID-00000000000002",
	Enabled:           false,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var multipleTagsSubscription = moira.SubscriptionData{
	ID:                "subscriptionID-00000000000003",
	Enabled:           true,
	Tags:              []string{"test-tag", "one-more-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}
