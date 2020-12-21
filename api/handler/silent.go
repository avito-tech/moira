package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/api/middleware"
)

func silent(router chi.Router) {
	router.Get("/", getSilentPatterns)
	router.Put("/", createSilentPattern)
	router.Post("/", updateSilentPattern)
	router.Delete("/", removeSilentPattern)
}

func createSilentPatternManager(request *http.Request) *controller.SilentPatternManager {
	return controller.CreateSilentPatternManager(middleware.GetConfig(request))
}

func getSilentPatterns(writer http.ResponseWriter, request *http.Request) {
	patternTypeInt, err := strconv.ParseInt(request.URL.Query().Get("type"), 10, 64)
	if err != nil {
		patternTypeInt = 0
	}
	patternType := moira.SilentPatternType(patternTypeInt)

	manager := createSilentPatternManager(request)
	patterns, err := manager.GetSilentPatterns(database, patternType)
	if err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}

	if err := render.Render(writer, request, patterns); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func createSilentPattern(writer http.ResponseWriter, request *http.Request) {
	silentPatterns := &dto.SilentPatternList{}
	if err := render.Bind(request, silentPatterns); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	login := middleware.GetLogin(request)
	manager := createSilentPatternManager(request)

	if err := manager.CreateSilentPatterns(database, silentPatterns, login); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	for _, silentPattern := range silentPatterns.List {
		silentPattern.Login = login
	}
	middleware.GetLoggerEntry(request).InfoE("Silent pattern(s) created", silentPatterns)
}

func updateSilentPattern(writer http.ResponseWriter, request *http.Request) {
	silentPatterns := &dto.SilentPatternList{}
	if err := render.Bind(request, silentPatterns); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	login := middleware.GetLogin(request)
	manager := createSilentPatternManager(request)

	if err := manager.UpdateSilentPatterns(database, silentPatterns, login); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	for _, silentPattern := range silentPatterns.List {
		silentPattern.Login = login
	}
	middleware.GetLoggerEntry(request).InfoE("Silent pattern(s) updated", silentPatterns)
}

func removeSilentPattern(writer http.ResponseWriter, request *http.Request) {
	manager := createSilentPatternManager(request)
	silentPatterns := &dto.SilentPatternList{}

	if err := render.Bind(request, silentPatterns); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	if err := manager.RemoveSilentPatterns(database, silentPatterns); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	login := middleware.GetLogin(request)
	for _, silentPattern := range silentPatterns.List {
		silentPattern.Login = login
	}
	middleware.GetLoggerEntry(request).InfoE("Silent pattern(s) deleted", silentPatterns)
}
