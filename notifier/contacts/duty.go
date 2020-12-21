package contacts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.avito.ru/DO/moira"
)

const (
	dutyCacheTTL = 5 * time.Minute
)

type dutyAPIData struct {
	Main   []moira.DutyItem
	Backup []moira.DutyItem
}

// resolveDuty finds the person on duty for a service and returns their login
func (decoder *Decoder) resolveDuty(service string) ([]moira.DutyItem, error) {
	cachedResult, err := decoder.db.GetServiceDuty(service)
	if err != nil {
		decoder.logger.ErrorF("Failed to resolve duty, service [%s]: %v", service, err)
	} else if time.Now().Sub(cachedResult.Timestamp) <= dutyCacheTTL {
		return cachedResult.Duty, nil
	}

	result, err := decoder.requestDutyApi(service)
	if err != nil {
		err = fmt.Errorf("ResolveDuty: %s", err.Error())
		return result, err
	}

	if len(result) == 0 {
		return result, ErrNobodyOnDuty{service: service}
	}

	go func(duty []moira.DutyItem) { // cache the data asynchronously
		err := decoder.db.UpdateServiceDuty(service, moira.DutyData{
			Duty:      duty,
			Timestamp: time.Now(),
		})
		if err != nil {
			decoder.logger.ErrorF("Error while saving duty, service %s: %s", service, err.Error())
		}
	}(result)

	return result, nil
}

func (decoder *Decoder) requestDutyApi(service string) ([]moira.DutyItem, error) {
	client := http.Client{}
	client.Timeout = 3 * time.Second
	url := decoder.dutyUrl + "/api/duty/" + service

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Token "+decoder.dutyAPIToken)
	req.Header.Add("User-Agent", "moira")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	decoder.logger.InfoE("Duty API returned some data", map[string]interface{}{
		"status": resp.StatusCode,
		"body":   string(respBody),
	})

	// duty.avito.ru returns a 0-byte JSON if there's nobody on duty
	if len(respBody) == 0 {
		return nil, nil
	}

	dutyAPIData := dutyAPIData{}
	if err = json.Unmarshal(respBody, &dutyAPIData); err != nil {
		decoder.logger.ErrorF("Bad json data received from url %s, err: %v", url, err)
		return nil, err
	}

	if len(dutyAPIData.Main) > 0 {
		return dutyAPIData.Main, nil
	} else {
		return dutyAPIData.Backup, nil
	}
}
