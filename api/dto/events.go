// nolint
package dto

import (
	"net/http"

	"go.avito.ru/DO/moira"
)

type EventsList struct {
	Page  int64                     `json:"page"`
	Size  int64                     `json:"size"`
	Total int64                     `json:"total"`
	List  []moira.NotificationEvent `json:"list"`
}

func (*EventsList) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
