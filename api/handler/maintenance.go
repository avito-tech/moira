package handler

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/pkg/errors"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
)

// maintenance wraps debug methods which expose maintenance values for silent pattern / tag and trigger
func maintenance(router chi.Router) {
	router.Get("/metrics", getMaintenanceMetrics)
	router.Get("/tags", getMaintenanceTags)
	router.Route("/trigger/{id}", func(router chi.Router) {
		router.Get("/", getMaintenanceTrigger)
	})
}

func getMaintenanceMetrics(writer http.ResponseWriter, request *http.Request) {
	maintenance, err := controller.GetMaintenanceSilent(database, moira.SPTMetric)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}

	if err := render.Render(writer, request, maintenance); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func getMaintenanceTags(writer http.ResponseWriter, request *http.Request) {
	maintenance, err := controller.GetMaintenanceSilent(database, moira.SPTTag)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}

	if err := render.Render(writer, request, maintenance); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func getMaintenanceTrigger(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")
	if id == "" {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(errors.New("'id' param must be set")))
	}

	maintenance, err := controller.GetMaintenanceTrigger(database, id)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}

	if err := render.Render(writer, request, maintenance); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}
