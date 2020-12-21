package notifier

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/notifier"
	"go.avito.ru/DO/moira/notifier/events"
	"go.avito.ru/DO/moira/notifier/notifications"
	"go.avito.ru/DO/moira/test-helpers"
)

var senderSettings = map[string]string{
	"type": "mega-sender",
}

var location, _ = time.LoadLocation("UTC")

var notifierConfig = notifier.Config{
	SendingTimeout:   time.Millisecond * 10,
	ResendingTimeout: time.Hour * 24,
	Location:         location,
}

var shutdown = make(chan bool)
var mockCtrl *gomock.Controller

var contact = moira.ContactData{
	ID:    "ContactID-000000000000001",
	Type:  "mega-sender",
	Value: "mail1@example.com",
}

var subscription = moira.SubscriptionData{
	ID:                "subscriptionID-00000000000001",
	Enabled:           true,
	Tags:              []string{"test-tag-1"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var trigger = moira.Trigger{
	ID:      uuid.NewV4().String(),
	Name:    "test trigger 1",
	Targets: []string{"test.target.1"},
	Tags:    []string{"test-tag-1"},
}

var triggerData = moira.TriggerData{
	ID:      trigger.ID,
	Name:    "test trigger 1",
	Targets: []string{"test.target.1"},
	Tags:    []string{"test-tag-1"},
}

var event = moira.NotificationEvent{
	Metric:    "generate.event.1",
	State:     moira.OK,
	OldState:  moira.WARN,
	TriggerID: trigger.ID,
}

var (
	logger          moira.Logger
	notifierMetrics *metrics.NotifierMetrics
)

func init() {
	test_helpers.InitTestLogging()

	logger = test_helpers.GetTestLogger()
	notifierMetrics = metrics.NewNotifierMetrics()
}

func TestNotifier(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	database := redis.NewDatabase(logger, redis.Config{Port: "6379", Host: "localhost"})
	database.SaveContact(&contact)
	database.SaveSubscription(&subscription)
	database.SaveTrigger(trigger.ID, &trigger)
	database.PushNotificationEvent(&event)
	notifier2 := notifier.NewNotifier(database, notifierConfig, notifierMetrics)
	sender := mock_moira_alert.NewMockSender(mockCtrl)
	sender.EXPECT().Init(senderSettings, location).Return(nil)
	notifier2.RegisterSender(senderSettings, sender)
	sender.EXPECT().SendEvents(gomock.Any(), contact, triggerData, false, false).Return(nil).Do(func(f ...interface{}) {
		fmt.Print("SendEvents called. End test")
		close(shutdown)
	})

	fetchEventsWorker := events.FetchEventsWorker{
		Database:  database,
		Logger:    logger,
		Metrics:   notifierMetrics,
		Scheduler: notifier.NewScheduler(database, notifierMetrics),

		Fetcher: func() (events moira.NotificationEvents, err error) {
			event, err := database.FetchNotificationEvent(false)
			return moira.NotificationEvents{event}, err
		},
	}

	fetchNotificationsWorker := notifications.FetchNotificationsWorker{
		Database: database,
		Logger:   logger,
		Notifier: notifier2,
	}

	fetchEventsWorker.Start()
	defer fetchEventsWorker.Stop()

	fetchNotificationsWorker.Start()
	defer fetchNotificationsWorker.Stop()

	waitTestEnd()
}

func waitTestEnd() {
	select {
	case <-shutdown:
		break
	case <-time.After(time.Second * 30):
		fmt.Print("Test timeout")
		close(shutdown)
		break
	}
}
