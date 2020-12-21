package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/api/middleware"
)

func subscription(router chi.Router) {
	router.Get("/", getUserSubscriptions)
	router.Get("/search", searchSubscriptions)
	router.Put("/", createSubscription)
	router.Route("/{subscriptionId}", func(router chi.Router) {
		router.Use(middleware.SubscriptionContext)
		router.Use(subscriptionFilter)
		router.Put("/", updateSubscription)
		router.Delete("/", removeSubscription)
		router.Put("/test", sendTestNotification)
	})
}

func getUserSubscriptions(writer http.ResponseWriter, request *http.Request) {
	userLogin := middleware.GetLogin(request)
	contacts, err := controller.GetUserSubscriptions(database, userLogin)
	if err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, contacts); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

// searchSubscriptions searches for subscriptions (including escalations) those match given contact value
func searchSubscriptions(writer http.ResponseWriter, request *http.Request) {
	contactValue := request.FormValue("contact")
	subscriptions, err := controller.GetSubscriptionsByContactValue(database, contactValue)
	if err != nil {
		_ = render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, subscriptions); err != nil {
		_ = render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func createSubscription(writer http.ResponseWriter, request *http.Request) {
	subscription := &dto.Subscription{}
	if err := render.Bind(request, subscription); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	userLogin := middleware.GetLogin(request)

	if err := controller.CreateSubscription(database, userLogin, subscription); err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, subscription); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}

	middleware.GetLoggerEntry(request).InfoE("Subscription created", subscription)
}

// subscriptionFilter is middleware for check subscription existence and user permissions
func subscriptionFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		contactID := middleware.GetSubscriptionId(request)
		userLogin := middleware.GetLogin(request)
		subscriptionData, err := controller.CheckUserPermissionsForSubscription(database, contactID, userLogin)
		if err != nil {
			render.Render(writer, request, err)
			return
		}
		ctx := context.WithValue(request.Context(), subscriptionKey, subscriptionData)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func updateSubscription(writer http.ResponseWriter, request *http.Request) {
	subscription := &dto.Subscription{}
	if err := render.Bind(request, subscription); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	subscriptionData := request.Context().Value(subscriptionKey).(moira.SubscriptionData)

	if err := controller.UpdateSubscription(database, subscriptionData.ID, subscriptionData.User, subscription); err != nil {
		render.Render(writer, request, err)
		return
	}
	middleware.GetLoggerEntry(request).InfoE("Subscription updated", subscription)

	if err := render.Render(writer, request, subscription); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func removeSubscription(writer http.ResponseWriter, request *http.Request) {
	login := middleware.GetLogin(request)
	logger := middleware.GetLoggerEntry(request)
	subscriptionId := middleware.GetSubscriptionId(request)

	logger.InfoE(fmt.Sprintf("Calling removeSubscription with subscriptionId = %s", subscriptionId), map[string]interface{}{
		"login":          login,
		"subscriptionId": subscriptionId,
	})

	// trace removed subscription to CH
	subscriptionData, err := controller.GetSubscriptionById(database, subscriptionId)
	if err != nil {
		logger.ErrorF("Failed to get subscription data for id %s: %v", subscriptionId, err)
		_ = render.Render(writer, request, err)
		return
	}
	logger.InfoE("Trace removed data", subscriptionData)

	err = controller.RemoveSubscription(database, subscriptionId)
	if err != nil {
		logger.ErrorF("Failed to remove subscription id %s: %v", subscriptionId, err)
		_ = render.Render(writer, request, err)
		return
	}
	logger.InfoF("Successfully remove subscription id %s", subscriptionId)
}

func sendTestNotification(writer http.ResponseWriter, request *http.Request) {
	subscriptionID := middleware.GetSubscriptionId(request)
	if err := controller.SendTestNotification(database, subscriptionID); err != nil {
		render.Render(writer, request, err)
	}
}
