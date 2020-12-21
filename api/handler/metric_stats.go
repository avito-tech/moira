package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
)

func metricStats(router chi.Router) {
	router.Get("/", getMetricStats)
}

func getMetricStats(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	filterTags := getRequestTags(request)
	onlyErrors := getOnlyProblemsFlag(request)
	intervalLength, err := getIntervalLength(request)
	if err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	metricStats, errorResponse := controller.GetMetricStats(database, intervalLength, onlyErrors, filterTags)
	if errorResponse != nil {
		render.Render(writer, request, errorResponse)
		return
	}

	if err := render.Render(writer, request, metricStats); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func getIntervalLength(request *http.Request) (int64, error) {
	intervalLengthStr := request.FormValue("intervalLength")
	if intervalLengthStr == "" {
		return 0, fmt.Errorf("intervalLength is required")
	}
	intervalLength, err := strconv.Atoi(intervalLengthStr)
	if err != nil {
		return 0, fmt.Errorf("intervalLength should be a number")
	}
	if intervalLength <= 0 {
		return 0, fmt.Errorf("intervalLength should be greater than 0")
	}
	return int64(intervalLength), nil
}
