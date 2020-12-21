package notifications

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/mock/notifier"
	"go.avito.ru/DO/moira/notifier"
	"go.avito.ru/DO/moira/test-helpers"
)

var logger = test_helpers.GetTestLogger()

func TestProcessScheduledEvent(t *testing.T) {
	subID2 := "subscriptionID-00000000000002"
	subID5 := "subscriptionID-00000000000005"
	subID7 := "subscriptionID-00000000000007"

	notification1 := moira.ScheduledNotification{
		Event: moira.NotificationEvent{
			SubscriptionID: &subID5,
			State:          moira.TEST,
		},
		Contact:   contact1,
		Throttled: false,
		Timestamp: 1441188915,
	}
	notification2 := moira.ScheduledNotification{
		Event: moira.NotificationEvent{
			SubscriptionID: &subID7,
			State:          moira.TEST,
			TriggerID:      "triggerID-00000000000001",
		},
		Contact:   contact2,
		Throttled: false,
		SendFail:  0,
		Timestamp: 1441188915,
	}
	notification3 := moira.ScheduledNotification{
		Event: moira.NotificationEvent{
			SubscriptionID: &subID2,
			State:          moira.TEST,
			TriggerID:      "triggerID-00000000000001",
		},
		Contact:   contact2,
		Throttled: false,
		SendFail:  0,
		Timestamp: 1441188915,
	}

	Convey("Two different notifications, should send two packages", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		mockNotifier := mock_notifier.NewMockNotifier(mockCtrl)
		worker := &FetchNotificationsWorker{
			Database: dataBase,
			Logger:   logger,
			Notifier: mockNotifier,
		}

		dataBase.EXPECT().FetchNotifications(gomock.Any()).Return([]*moira.ScheduledNotification{
			&notification1,
			&notification2,
		}, nil)
		dataBase.EXPECT().GetGlobalSettings()

		pkg1 := notifier.NotificationPackage{
			Trigger:    notification1.Trigger,
			Throttled:  notification1.Throttled,
			Contact:    notification1.Contact,
			DontResend: false,
			FailCount:  0,
			Events: []moira.NotificationEvent{
				notification1.Event,
			},
		}
		pkg2 := notifier.NotificationPackage{
			Trigger:    notification2.Trigger,
			Throttled:  notification2.Throttled,
			Contact:    notification2.Contact,
			DontResend: false,
			FailCount:  0,
			Events: []moira.NotificationEvent{
				notification2.Event,
			},
		}
		mockNotifier.EXPECT().Send(&pkg1, gomock.Any())
		mockNotifier.EXPECT().Send(&pkg2, gomock.Any())
		err := worker.processScheduledNotifications()
		So(err, ShouldBeEmpty)
	})

	Convey("Two same notifications, should send one package", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		mockNotifier := mock_notifier.NewMockNotifier(mockCtrl)
		worker := &FetchNotificationsWorker{
			Database: dataBase,
			Logger:   logger,
			Notifier: mockNotifier,
		}

		dataBase.EXPECT().FetchNotifications(gomock.Any()).Return([]*moira.ScheduledNotification{
			&notification2,
			&notification3,
		}, nil)
		dataBase.EXPECT().GetGlobalSettings()

		pkg := notifier.NotificationPackage{
			Trigger:    notification2.Trigger,
			Throttled:  notification2.Throttled,
			Contact:    notification2.Contact,
			DontResend: false,
			FailCount:  0,
			Events: []moira.NotificationEvent{
				notification2.Event,
				notification3.Event,
			},
		}

		mockNotifier.EXPECT().Send(&pkg, gomock.Any())
		err := worker.processScheduledNotifications()
		So(err, ShouldBeEmpty)
	})
}

func TestGoRoutine(t *testing.T) {
	subID5 := "subscriptionID-00000000000005"

	notification1 := moira.ScheduledNotification{
		Event: moira.NotificationEvent{
			SubscriptionID: &subID5,
			State:          moira.TEST,
		},
		Contact:   contact1,
		Throttled: false,
		Timestamp: 1441188915,
	}

	pkg := notifier.NotificationPackage{
		Trigger:    notification1.Trigger,
		Throttled:  notification1.Throttled,
		Contact:    notification1.Contact,
		DontResend: false,
		FailCount:  0,
		Events: []moira.NotificationEvent{
			notification1.Event,
		},
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	mockNotifier := mock_notifier.NewMockNotifier(mockCtrl)

	worker := &FetchNotificationsWorker{
		Database: dataBase,
		Logger:   logger,
		Notifier: mockNotifier,
	}

	shutdown := make(chan bool)
	dataBase.EXPECT().FetchNotifications(gomock.Any()).Return([]*moira.ScheduledNotification{&notification1}, nil)
	dataBase.EXPECT().GetGlobalSettings()
	mockNotifier.EXPECT().Send(&pkg, gomock.Any()).Do(func(f ...interface{}) { close(shutdown) })
	mockNotifier.EXPECT().StopSenders()

	worker.Start()
	waitTestEnd(shutdown, worker)
}

func waitTestEnd(shutdown chan bool, worker *FetchNotificationsWorker) {
	select {
	case <-shutdown:
		_ = worker.Stop()
		break
	case <-time.After(time.Second * 10):
		close(shutdown)
		break
	}
}

var contact1 = moira.ContactData{
	ID:    "ContactID-000000000000001",
	Type:  "email",
	Value: "mail1@example.com",
}

var contact2 = moira.ContactData{
	ID:    "ContactID-000000000000006",
	Type:  "unknown",
	Value: "no matter",
}
