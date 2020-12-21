// nolint
package dto

import (
	"fmt"
	"net/http"

	"go.avito.ru/DO/moira"
)

type SubscriptionList struct {
	List []moira.SubscriptionData `json:"list"`
}

func (*SubscriptionList) Render(http.ResponseWriter, *http.Request) error {
	return nil
}

type Subscription moira.SubscriptionData

func (subscription *Subscription) Bind(r *http.Request) error {
	if len(subscription.Tags) == 0 {
		return fmt.Errorf("Subscription must have tags")
	}
	if len(subscription.Contacts) == 0 {
		return fmt.Errorf("Subscription must have contacts")
	}
	return nil
}

func (*Subscription) Render(http.ResponseWriter, *http.Request) error {
	return nil
}

type EscalationFiltered struct {
	ID              string              `json:"id"`
	Contacts        []moira.ContactData `json:"contacts"`
	OffsetInMinutes int64               `json:"offset_in_minutes"`
}

type SubscriptionFiltered struct {
	ID      string   `json:"id"`
	Enabled bool     `json:"enabled"`
	Tags    []string `json:"tags"`
	User    string   `json:"user"`

	Contacts    []moira.ContactData  `json:"contacts"`
	Escalations []EscalationFiltered `json:"escalations"`

	MatchedEsc []bool `json:"matched_esc"`
	MatchedSub bool   `json:"matched_sub"`
}

type SubscriptionFilteredList struct {
	List []SubscriptionFiltered `json:"list"`
}

func (*SubscriptionFilteredList) Render(http.ResponseWriter, *http.Request) error {
	return nil
}
