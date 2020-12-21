package notifier

import (
	"fmt"
	"strings"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/senders/mail"
	"go.avito.ru/DO/moira/senders/pushover"
	"go.avito.ru/DO/moira/senders/script"
	"go.avito.ru/DO/moira/senders/slack"
	"go.avito.ru/DO/moira/senders/telegram"
	"go.avito.ru/DO/moira/senders/twilio"
	"go.avito.ru/DO/moira/senders/webhook"
)

// RegisterSenders watch on senders config and register all configured senders
func (notifier *StandardNotifier) RegisterSenders(connector moira.Database) error {
	for _, senderSettings := range notifier.config.Senders {
		senderSettings["front_uri"] = notifier.config.FrontURL
		switch senderSettings["type"] {
		case "pushover":
			if err := notifier.RegisterSender(senderSettings, &pushover.Sender{}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "slack":
			if err := notifier.RegisterSender(senderSettings, &slack.Sender{DataBase: connector}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "mail":
			if err := notifier.RegisterSender(senderSettings, &mail.Sender{}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "script":
			if err := notifier.RegisterSender(senderSettings, &script.Sender{}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "telegram":
			if err := notifier.RegisterSender(senderSettings, &telegram.Sender{DataBase: connector}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "twilio sms":
			if err := notifier.RegisterSender(senderSettings, &twilio.Sender{}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "twilio voice":
			if err := notifier.RegisterSender(senderSettings, &twilio.Sender{}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		case "webhook":
			if err := notifier.RegisterSender(senderSettings, &webhook.Sender{}); err != nil {
				notifier.logger.Fatal(fmt.Sprintf("Can not register sender %s: %s", senderSettings["type"], err))
			}
		default:
			return fmt.Errorf("Unknown sender type [%s]", senderSettings["type"])
		}
	}

	// also add metrics for slack delayed action worker
	notifier.addSenderMetrics(slack.DelayedActionWorkerId)

	return nil
}

// RegisterSender adds sender for notification type and registers metrics
func (notifier *StandardNotifier) RegisterSender(senderSettings map[string]string, sender moira.Sender) error {
	var senderIdent string
	if senderSettings["type"] == "script" {
		senderIdent = senderSettings["name"]
	} else {
		senderIdent = senderSettings["type"]
	}

	err := sender.Init(senderSettings, notifier.config.Location)
	if err != nil {
		return fmt.Errorf("Don't initialize sender [%s], err [%s]", senderIdent, err.Error())
	}

	ch := make(chan NotificationPackage)
	notifier.senders[senderIdent] = ch
	notifier.addSenderMetrics(senderIdent)
	notifier.waitGroup.Add(1)

	go notifier.run(sender, ch)

	notifier.logger.Info(fmt.Sprintf("Sender %s registered", senderIdent))
	return nil
}

// StopSenders close all sending channels
func (notifier *StandardNotifier) StopSenders() {
	for _, ch := range notifier.senders {
		close(ch)
	}

	notifier.senders = make(map[string]chan NotificationPackage)
	notifier.logger.Info("Waiting senders finish...")
	notifier.waitGroup.Wait()
	notifier.logger.Info("Moira Notifier Senders stopped")
}

func (notifier *StandardNotifier) addSenderMetrics(id string) {
	notifier.metrics.SendersOkMetrics.AddMetric(id, fmt.Sprintf("notifier.%s.sends_ok", getGraphiteSenderIdent(id)))
	notifier.metrics.SendersFailedMetrics.AddMetric(id, fmt.Sprintf("notifier.%s.sends_failed", getGraphiteSenderIdent(id)))
}

func getGraphiteSenderIdent(ident string) string {
	return strings.Replace(ident, " ", "_", -1)
}
