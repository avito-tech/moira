package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
)

// Sender implements moira sender interface via webhook
type Sender struct{}

type JsonMessage struct {
	Events  *moira.NotificationEvents `json:"events"`
	Trigger *moira.TriggerData        `json:"trigger"`
}

// Init read yaml config
func (sender *Sender) Init(_ map[string]string, _ *time.Location) error {
	return nil
}

// SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData, _, _ bool) error {
	webhookUrl := contact.Value
	msg := &JsonMessage{Events: &events, Trigger: &trigger}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("unable to marshal json")
	}
	logging.GetLogger(trigger.ID).Debug(fmt.Sprintf("Calling webhook with url %s", webhookUrl))
	return do(contact.Value, payload)
}

func do(webhookUrl string, payload []byte) error {
	parsedURL, err := url.Parse(webhookUrl)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", parsedURL.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-Source", "moira")

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("failed to call webhook. Returned statuscode %v body %s", res.StatusCode, body)
	}
	return nil

}
