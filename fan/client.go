package fan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.avito.ru/DO/moira"
)

type Request struct {
	Event       moira.NotificationEvent `json:"event"`
	TriggerData moira.TriggerData       `json:"trigger_data"`
}

type Response struct {
	Done bool   `json:"done"`
	ID   string `json:"id"`

	Event       *moira.NotificationEvent `json:"event"`
	TriggerData *moira.TriggerData       `json:"trigger_data"`
	Error       interface{}              `json:"error"`
}

type Client struct {
	URL string

	httpClient *http.Client
}

func NewClient(URL string) Client {
	return Client{
		URL: URL,

		httpClient: &http.Client{
			Timeout: 1 * time.Second,
		},
	}
}

func (client Client) SendRequest(request Request) (taskID string, err error) {
	encodedRequest, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("PUT", client.URL+"/request", bytes.NewBuffer(encodedRequest))
	if err != nil {
		return "", err
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var responseData Response
	err = json.Unmarshal(responseBytes, &responseData)
	if err != nil {
		return "", err
	}

	if responseData.ID == "" {
		return "", fmt.Errorf("empty ID returned")
	}

	return responseData.ID, nil
}

func (client Client) CheckProgress(taskID string) (Response, error) {
	req, err := http.NewRequest("GET", client.URL+"/request/"+taskID, nil)
	if err != nil {
		return Response{}, err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return Response{}, err
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	var response Response
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return Response{}, err
	}

	return response, nil
}

func (client Client) ApplyFallbacks(event *moira.NotificationEvent, triggerData *moira.TriggerData, saturation []moira.Saturation) {
	// add a tag indicating that saturation failed
	const SaturationFailedTag = "fan-saturation-failed"
	var shouldAddTag = true
	for _, tag := range triggerData.Tags {
		if tag == SaturationFailedTag {
			shouldAddTag = false
			break
		}
	}
	if shouldAddTag {
		triggerData.Tags = append(triggerData.Tags, SaturationFailedTag)
	}

	// TODO
	return
}
