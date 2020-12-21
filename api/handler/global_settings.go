package handler

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/api/middleware"
)

func globalSettings(router chi.Router) {
	router.Get("/", getGlobalSettings)
	router.Put("/", setGlobalSettings)
}

func checkPermissions(request *http.Request) *api.ErrorResponse {
	userName := middleware.GetLogin(request)
	isSuperUser := superUsers[userName]

	if !isSuperUser {
		errMessage := fmt.Sprintf("User \"%s\" is forbidden to request or modify global settings", userName)
		return api.ErrorForbidden(errMessage)
	} else {
		return nil
	}
}

func getGlobalSettings(writer http.ResponseWriter, request *http.Request) {
	if errResponse := checkPermissions(request); errResponse != nil {
		_ = render.Render(writer, request, errResponse)
	} else if globalSettings, errResponse := controller.GetGlobalSettings(database); errResponse != nil {
		_ = render.Render(writer, request, errResponse)
	} else if err := render.Render(writer, request, globalSettings); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
	}
}

func setGlobalSettings(writer http.ResponseWriter, request *http.Request) {
	newSettings := &dto.GlobalSettings{}
	if errResponse := checkPermissions(request); errResponse != nil {
		_ = render.Render(writer, request, errResponse)
	} else if err := render.Bind(request, newSettings); err != nil {
		_ = render.Render(writer, request, api.ErrorInternalServer(err))
	} else if errResponse := controller.SetGlobalSettings(database, newSettings); errResponse != nil {
		_ = render.Render(writer, request, errResponse)
	}
}
