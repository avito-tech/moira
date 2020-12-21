package handler

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/middleware"
)

func event(router chi.Router) {
	router.With(middleware.TriggerContext, middleware.Paginate(0, 100)).Get("/{triggerId}", func(writer http.ResponseWriter, request *http.Request) {
		triggerID := middleware.GetTriggerID(request)
		size := middleware.GetSize(request)
		page := middleware.GetPage(request)

		eventsList, err := controller.GetTriggerEvents(database, triggerID, page, size)
		if err != nil {
			render.Render(writer, request, err)
			return
		}
		if err := render.Render(writer, request, eventsList); err != nil {
			render.Render(writer, request, api.ErrorRender(err))
		}
	})
}
