package checker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	pb "github.com/go-graphite/carbonapi/carbonzipperpb3"
	et "github.com/go-graphite/carbonapi/expr/types"

	"go.avito.ru/DO/moira/target"
)

func (triggerChecker *TriggerChecker) prepareGraphiteRequest(url string, from, until int64, targets []string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("format", "protobuf")
	q.Add("from", strconv.FormatInt(from, 10))
	q.Add("until", strconv.FormatInt(until, 10))
	for _, t := range targets {
		q.Add("target", t)
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
}

func (triggerChecker *TriggerChecker) makeGraphiteRequest(req *http.Request) (pb.MultiFetchResponse, error) {
	var pbResp pb.MultiFetchResponse
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return pbResp, err
	}
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = fmt.Errorf("bad response status %d: %s", resp.StatusCode, string(body))
		return pbResp, err
	}
	err = pbResp.Unmarshal(body)

	if len(pbResp.Errors) > 0 {
		triggerChecker.logger.ErrorE(
			"pull trigger: carbonapi returned errors",
			map[string]interface{}{
				"TriggerID":      triggerChecker.TriggerID,
				"URL":            req.URL.String(),
				"Carbonapi-UUID": resp.Header.Get("X-Carbonapi-UUID"),
				"Errors":         pbResp.Errors,
			},
		)
	}
	return pbResp, err
}

func (triggerChecker *TriggerChecker) convertGraphiteResponse(r pb.MultiFetchResponse) []*target.TimeSeries {
	ts := make([]*target.TimeSeries, len(r.Metrics))
	for i, m := range r.Metrics {
		md := et.MetricData{FetchResponse: *m}
		t := &target.TimeSeries{MetricData: md, Wildcard: false} // TODO check wildcard
		ts[i] = t
	}
	return ts
}

func (triggerChecker *TriggerChecker) PullRemote(url string, from, until int64, targets []string) ([]*target.TimeSeries, error) {
	req, err := triggerChecker.prepareGraphiteRequest(url, from, until, targets)
	if err != nil {
		return nil, err
	}
	resp, err := triggerChecker.makeGraphiteRequest(req)
	if err != nil {
		return nil, err
	}
	return triggerChecker.convertGraphiteResponse(resp), nil
}
