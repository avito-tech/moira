package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/go-graphite/carbonapi/date"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/api/middleware"
	"go.avito.ru/DO/moira/expression"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/target"
)

func trigger(router chi.Router) {
	router.Use(middleware.TriggerContext)
	router.Put("/", updateTrigger)
	router.Get("/", getTrigger)
	router.Delete("/", removeTrigger)
	router.Get("/state", getTriggerState)
	router.Route("/throttling", func(router chi.Router) {
		router.Get("/", getTriggerThrottling)
		router.Delete("/", deleteThrottling)
	})
	router.Route("/metrics", func(router chi.Router) {
		router.With(middleware.DateRange("-10minutes", "now")).Get("/", getTriggerMetrics)
		router.Delete("/", deleteTriggerMetric)
	})
	router.Put("/maintenance", setMetricsMaintenance)
	router.Put("/triggerMaintenance", setTriggerMaintenance)
	router.Delete("/escalations", ackEscalations)
	router.Post("/ackEscalations", ackMetricEscalations)

	router.Post("/_unacknowledgedMessages", getUnacknowledgedMessages)
}

func updateTrigger(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
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
	response, err := controller.UpdateTrigger(
		database, triggerInheritanceDatabase,
		&trigger.TriggerModel, triggerID, timeSeriesNames,
	)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}

	logging.GetLogger(triggerID).InfoE("Trigger updated", map[string]interface{}{
		"login":      middleware.GetLogin(request),
		"trigger":    trigger,
		"user_agent": request.Header.Get("User-Agent"),
	})

	if err := render.Render(writer, request, response); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func removeTrigger(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	err := controller.RemoveTrigger(database, triggerID)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}

	logging.GetLogger(triggerID).InfoE("Trigger removed", map[string]interface{}{
		"login":      middleware.GetLogin(request),
		"trigger_id": triggerID,
		"user_agent": request.Header.Get("User-Agent"),
	})
}

func getTrigger(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	if triggerID == "testlog" {
		panic("Test for multi line logs")
	}
	trigger, err := controller.GetTrigger(database, triggerID)
	if err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, trigger); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}

func getTriggerState(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	triggerState, err := controller.GetTriggerLastCheck(database, triggerID)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, triggerState); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
	}
}

func getTriggerThrottling(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	triggerState, err := controller.GetTriggerThrottling(database, triggerID)
	if err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, triggerState); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}

func deleteThrottling(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	err := controller.DeleteTriggerThrottling(database, triggerID)
	if err != nil {
		render.Render(writer, request, err)
		return
	}

	logging.GetLogger(triggerID).InfoE("Trigger throttling removed", map[string]interface{}{
		"login":      middleware.GetLogin(request),
		"trigger_id": triggerID,
	})
}

func getTriggerMetrics(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	fromStr := middleware.GetFromStr(request)
	toStr := middleware.GetToStr(request)
	from := date.DateParamToEpoch(fromStr, "UTC", 0, time.UTC)
	if from == 0 {
		render.Render(writer, request, api.ErrorInvalidRequest(fmt.Errorf("Can not parse from: %s", fromStr)))
		return
	}
	to := date.DateParamToEpoch(toStr, "UTC", 0, time.UTC)
	if to == 0 {
		render.Render(writer, request, api.ErrorInvalidRequest(fmt.Errorf("Can not parse to: %v", to)))
		return
	}
	triggerMetrics, err := controller.GetTriggerMetrics(database, int64(from), int64(to), triggerID)
	if err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, &triggerMetrics); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}

func deleteTriggerMetric(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	metricName := request.URL.Query().Get("name")
	if metricName == "" {
		render.Render(writer, request, api.ErrorInvalidRequest(fmt.Errorf("Metric name can not be empty")))
		return
	}
	if err := controller.DeleteTriggerMetric(database, metricName, triggerID); err != nil {
		render.Render(writer, request, err)
		return
	}

	logging.GetLogger(triggerID).InfoE("Trigger metric deleted", map[string]interface{}{
		"login":       middleware.GetLogin(request),
		"metric_name": metricName,
		"trigger_id":  triggerID,
	})
}

func setMetricsMaintenance(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	userLogin := middleware.GetLogin(request)

	metricsMaintenance := dto.MetricsMaintenance{}
	if err := render.Bind(request, &metricsMaintenance); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	err := controller.SetMetricsMaintenance(database, triggerID, metricsMaintenance)
	if err != nil {
		_ = render.Render(writer, request, err)
	}

	logger := logging.GetLogger(triggerID)
	logger.InfoE(
		fmt.Sprintf("User %s has set metric maintenance for trigger id %s with err = %v", userLogin, triggerID, err),
		map[string]interface{}{
			"maintenance": metricsMaintenance,
			"user":        userLogin,
		},
	)
}

func setTriggerMaintenance(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	userLogin := middleware.GetLogin(request)

	triggerMaintenance := dto.TriggerMaintenance{}
	if err := render.Bind(request, &triggerMaintenance); err != nil {
		_ = render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	err := controller.SetTriggerMaintenance(database, triggerID, triggerMaintenance.Until)
	if err != nil {
		_ = render.Render(writer, request, err)
	}

	logger := logging.GetLogger(triggerID)
	logger.InfoE(
		fmt.Sprintf("User %s has set trigger maintenance for trigger id %s with err = %v", userLogin, triggerID, err),
		map[string]interface{}{
			"maintenance": triggerMaintenance,
			"user":        userLogin,
		},
	)
}

func ackEscalations(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	logger := logging.GetLogger(triggerID)

	err := controller.AckEscalations(database, triggerID)
	if err != nil {
		_ = render.Render(writer, request, err)
		logger.ErrorE("Failed to ack trigger escalation", map[string]interface{}{
			"trigger": triggerID,
			"error":   err,
		})
		return
	}

	login := middleware.GetLogin(request)
	logger.InfoE("Trigger escalations acknowledged", map[string]interface{}{
		"login":      login,
		"trigger_id": triggerID,
	})
}

func ackMetricEscalations(writer http.ResponseWriter, request *http.Request) {
	triggerID := middleware.GetTriggerID(request)
	logger := logging.GetLogger(triggerID)

	requestData := &dto.AckMetricEscalationsRequest{}
	if err := render.Bind(request, requestData); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		logger.ErrorE("Failed to parse ack metric escalation request", map[string]interface{}{
			"trigger": triggerID,
			"error":   err.Error(),
		})
		return
	}

	err := controller.AckEscalationsMetrics(database, triggerID, requestData.Metrics)
	if err != nil {
		_ = render.Render(writer, request, err)
		logger.ErrorE("Failed to ack metric escalation", map[string]interface{}{
			"trigger": triggerID,
			"error":   err,
		})
		return
	}

	login := middleware.GetLogin(request)
	logger.InfoE("Trigger escalations acknowledged", map[string]interface{}{
		"login":      login,
		"trigger_id": triggerID,
		"metrics":    requestData.Metrics,
	})
}

func getUnacknowledgedMessages(writer http.ResponseWriter, request *http.Request) {
	requestData := &dto.UnacknowledgedMessagesRequest{}
	if err := render.Bind(request, requestData); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}

	triggerID := middleware.GetTriggerID(request)
	messages, err := controller.GetUnacknowledgedMessages(database, triggerID, requestData.Metrics)
	if err != nil {
		render.Render(writer, request, err)
	}
	if err := render.Render(writer, request, &messages); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}
