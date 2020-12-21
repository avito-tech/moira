package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/middleware"
)

func notification(router chi.Router) {
	router.Get("/", getNotification)
	router.Delete("/", deleteNotification)
}

func getNotification(writer http.ResponseWriter, request *http.Request) {
	start, err := strconv.ParseInt(request.URL.Query().Get("start"), 10, 64)
	if err != nil {
		start = 0
	}
	end, err := strconv.ParseInt(request.URL.Query().Get("end"), 10, 64)
	if err != nil {
		end = -1
	}

	notifications, errorResponse := controller.GetNotifications(database, start, end)
	if errorResponse != nil {
		render.Render(writer, request, errorResponse)
		return
	}
	if err := render.Render(writer, request, notifications); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}

func deleteNotification(writer http.ResponseWriter, request *http.Request) {
	notificationKey := request.URL.Query().Get("id")
	if notificationKey == "" {
		render.Render(writer, request, api.ErrorInvalidRequest(fmt.Errorf("Notification id can not be empty")))
		return
	}

	notifications, errorResponse := controller.DeleteNotification(database, notificationKey)
	if errorResponse != nil {
		render.Render(writer, request, errorResponse)
		return
	}

	login := middleware.GetLogin(request)
	middleware.GetLoggerEntry(request).InfoE("Notification deleted", map[string]interface{}{
		"id":    notificationKey,
		"login": login,
		"qty":   notifications.Result,
	})

	if err := render.Render(writer, request, notifications); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}
