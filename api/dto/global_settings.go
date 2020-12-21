package dto

import (
	"net/http"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api/middleware"
)

type GlobalSettings moira.GlobalSettings

func (globalSettings *GlobalSettings) Bind(request *http.Request) error {
	globalSettings.Notifications.Author = middleware.GetLogin(request)
	return nil
}

func (globalSettings *GlobalSettings) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
