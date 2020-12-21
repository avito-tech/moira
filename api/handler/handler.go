package handler

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/rs/cors"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	moira_middle "go.avito.ru/DO/moira/api/middleware"
)

var database moira.Database
var triggerInheritanceDatabase moira.TriggerInheritanceDatabase
var superUsers = make(map[string]bool)

const contactKey moira_middle.ContextKey = "contact"
const subscriptionKey moira_middle.ContextKey = "subscription"

// NewHandler creates new api handler request uris based on github.com/go-chi/chi
func NewHandler(
	db moira.Database,
	triggerInheritanceDb moira.TriggerInheritanceDatabase,
	log moira.Logger,
	config *api.Config,
	configFileContent []byte,
	appVersion string,
) http.Handler {
	database = db
	triggerInheritanceDatabase = triggerInheritanceDb

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(moira_middle.UserContext)
	router.Use(moira_middle.RequestLogger(log))
	router.Use(moira_middle.AppVersion(appVersion))
	router.Use(middleware.NoCache)

	router.NotFound(notFoundHandler)
	router.MethodNotAllowed(methodNotAllowedHandler)

	router.Route("/api", func(router chi.Router) {
		router.Use(moira_middle.DatabaseContext(database))
		router.Use(moira_middle.ConfigContext(*config))
		router.Get("/config", webConfig(configFileContent))
		router.Route("/user", user)
		router.Route("/trigger", triggers)
		router.Route("/tag", tag)
		router.Route("/pattern", pattern)
		router.Route("/event", event)
		router.Route("/contact", contact)
		router.Route("/subscription", subscription)
		router.Route("/notification", notification)
		router.Route("/silent-pattern", silent)
		router.Route("/global-settings", globalSettings)
		router.Route("/stats/metrics", metricStats)
		router.Route("/maintenance", maintenance)
	})

	if config.EnableCORS {
		return cors.AllowAll().Handler(router)
	}

	for _, userLogin := range config.SuperUsers {
		superUsers[userLogin] = true
	}

	return router
}

func webConfig(content []byte) http.HandlerFunc {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if content == nil {
			render.Render(writer, request, api.ErrorInternalServer(fmt.Errorf("Web config file was not loaded")))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write(content)
	})
}

func notFoundHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(404)
	render.Render(writer, request, api.ErrNotFound)
}

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(405)
	render.Render(writer, request, api.ErrMethodNotAllowed)
}
