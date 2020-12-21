package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/api/middleware"
	"go.avito.ru/DO/moira/expression"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/target"
)

func triggers(router chi.Router) {
	router.Get("/", getAllTriggers)
	router.Put("/", createTrigger)
	router.With(middleware.Paginate(0, 10)).Get("/page", getTriggersPage)
	router.Route("/{triggerId}", trigger)
}

func getAllTriggers(writer http.ResponseWriter, request *http.Request) {
	triggersList, errorResponse := controller.GetAllTriggers(database)
	if errorResponse != nil {
		render.Render(writer, request, errorResponse)
		return
	}

	if err := render.Render(writer, request, triggersList); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func createTrigger(writer http.ResponseWriter, request *http.Request) {
	trigger := &dto.Trigger{}
	if err := render.Bind(request, trigger); err != nil {
		switch err.(type) {
		case target.ErrParseExpr, target.ErrEvalExpr, target.ErrUnknownFunction:
			_ = render.Render(writer, request, api.ErrorInvalidRequest(fmt.Errorf("Invalid graphite targets: %s", err.Error())))
		case expression.ErrInvalidExpression:
			_ = render.Render(writer, request, api.ErrorInvalidRequest(fmt.Errorf("Invalid expression: %s", err.Error())))
		default:
			_ = render.Render(writer, request, api.ErrorInternalServer(err))
		}
		return
	}

	timeSeriesNames := middleware.GetTimeSeriesNames(request)
	response, err := controller.CreateTrigger(database, triggerInheritanceDatabase, &trigger.TriggerModel, timeSeriesNames)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}

	logging.GetLogger(trigger.ID).InfoE("Trigger created", map[string]interface{}{
		"login":      middleware.GetLogin(request),
		"trigger":    trigger,
		"user_agent": request.Header.Get("User-Agent"),
	})

	if err := render.Render(writer, request, response); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func getTriggersPage(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	filterName := getTriggerName(request)
	filterTags := getRequestTags(request)
	onlyErrors := getOnlyProblemsFlag(request)

	page := middleware.GetPage(request)
	size := middleware.GetSize(request)

	triggersList, errorResponse := controller.GetTriggerPage(database, page, size, onlyErrors, filterTags, filterName)
	if errorResponse != nil {
		render.Render(writer, request, errorResponse)
		return
	}

	if err := render.Render(writer, request, triggersList); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func getOnlyProblemsFlag(request *http.Request) bool {
	onlyProblemsStr := request.FormValue("onlyProblems")
	if onlyProblemsStr != "" {
		onlyProblems, _ := strconv.ParseBool(onlyProblemsStr)
		return onlyProblems
	}
	return false
}

func getRequestTags(request *http.Request) []string {
	var filterTags []string
	i := 0
	for {
		tag := request.FormValue(fmt.Sprintf("tags[%v]", i))
		if tag == "" {
			break
		}
		filterTags = append(filterTags, tag)
		i++
	}
	return filterTags
}

func getTriggerName(request *http.Request) string {
	return request.FormValue("triggerName")
}
