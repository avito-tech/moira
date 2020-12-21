package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/controller"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/api/middleware"
)

func contact(router chi.Router) {
	router.Get("/", getAllContacts)
	router.Put("/", createNewContact)
	router.Route("/{contactId}", func(router chi.Router) {
		router.Use(middleware.ContactContext)
		router.Use(contactFilter)
		router.Put("/", updateContact)
		router.Delete("/", removeContact)
		router.Post("/test", sendTestContactNotification)
	})
}

func getAllContacts(writer http.ResponseWriter, request *http.Request) {
	contacts, err := controller.GetAllContacts(database)
	if err != nil {
		render.Render(writer, request, err)
		return
	}

	if err := render.Render(writer, request, contacts); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func createNewContact(writer http.ResponseWriter, request *http.Request) {
	contact := &dto.Contact{}
	if err := render.Bind(request, contact); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	userLogin := middleware.GetLogin(request)

	if err := controller.CreateContact(database, contact, userLogin); err != nil {
		render.Render(writer, request, err)
		return
	}
	middleware.GetLoggerEntry(request).InfoE("Contact created", contact)

	if err := render.Render(writer, request, contact); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

// contactFilter is middleware for check contact existence and user permissions
func contactFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		contactID := middleware.GetContactID(request)
		userLogin := middleware.GetLogin(request)
		contactData, err := controller.CheckUserPermissionsForContact(database, contactID, userLogin)
		if err != nil {
			render.Render(writer, request, err)
			return
		}
		ctx := context.WithValue(request.Context(), contactKey, contactData)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func updateContact(writer http.ResponseWriter, request *http.Request) {
	contactDTO := dto.Contact{}
	if err := render.Bind(request, &contactDTO); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	contactData := request.Context().Value(contactKey).(moira.ContactData)

	contactDTO, err := controller.UpdateContact(database, contactDTO, contactData)
	if err != nil {
		render.Render(writer, request, err)
		return
	}
	middleware.GetLoggerEntry(request).InfoE("Contact updated", contactDTO)

	if err := render.Render(writer, request, &contactDTO); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
	}
}

func removeContact(writer http.ResponseWriter, request *http.Request) {
	contactData := request.Context().Value(contactKey).(moira.ContactData)
	err := controller.RemoveContact(database, contactData.ID, contactData.User)
	if err != nil {
		render.Render(writer, request, err)
		return
	}

	middleware.GetLoggerEntry(request).InfoE("Contact removed", contactData)
}

func sendTestContactNotification(writer http.ResponseWriter, request *http.Request) {
	contactID := middleware.GetContactID(request)
	err := controller.SendTestContactNotification(database, contactID)
	if err != nil {
		render.Render(writer, request, err)
	}
}
