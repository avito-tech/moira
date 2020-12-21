package dto

import (
	"net/http"
)

type MetricStatModel struct {
	Metric       string       `json:"metric"`
	Trigger      TriggerModel `json:"trigger"`
	ErrorCount   int64        `json:"error_count"`
	CurrentState string       `json:"current_state"`
}

type MetricStats struct {
	List []*MetricStatModel `json:"list"`
}

func (*MetricStats) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
