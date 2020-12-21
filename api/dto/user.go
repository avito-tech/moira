// nolint
package dto

import (
	"net/http"

	"go.avito.ru/DO/moira"
)

type User struct {
	Login       string `json:"login"`
	IsSuperUser bool   `json:"isSuperUser"`
}

type UserSettings struct {
	User
	Contacts      []moira.ContactData      `json:"contacts"`
	Subscriptions []moira.SubscriptionData `json:"subscriptions"`
}

func (*User) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (*UserSettings) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
