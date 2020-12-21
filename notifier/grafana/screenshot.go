package grafana

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func MakeScreenshot(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code")
		body, _ := ioutil.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to make screenshot. Returned statuscode %v body %s", res.StatusCode, body)
	}
	return ioutil.ReadAll(res.Body)
}
