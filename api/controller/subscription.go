package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-graphite/carbonapi/date"
	"github.com/satori/go.uuid"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/database"
)

func GetSubscriptionsByContactValue(database moira.Database, contactValue string) (*dto.SubscriptionFilteredList, *api.ErrorResponse) {
	contactValue = strings.ToLower(contactValue)
	if len(contactValue) < 3 {
		return nil, api.ErrorInvalidRequest(errors.New("\"contact\" must be at least 3 bytes long"))
	}

	result := &dto.SubscriptionFilteredList{
		List: nil,
	}

	// all existing contacts
	contacts, err := database.GetAllContacts()
	if err != nil {
		return nil, api.ErrorInternalServer(fmt.Errorf("Failed to get contacts data: %v", err))
	}

	// filter all contacts by value
	contactsAllMap := make(map[string]moira.ContactData, len(contacts))
	contactsMatchedSet := make(map[string]bool, len(contacts))
	usersSet := make(map[string]bool, len(contacts))
	for _, contact := range contacts {
		if contact == nil || contact.ID == "" {
			continue
		}

		contactsAllMap[contact.ID] = *contact
		if strings.Contains(strings.ToLower(contact.Value), contactValue) {
			contactsMatchedSet[contact.ID] = true
			usersSet[contact.User] = true
		}
	}

	// no contact has matched - hence no need to hit DB for subscriptions
	if len(contactsMatchedSet) == 0 {
		return result, nil
	}

	// search for all subscriptions of found users
	subscriptions := make([]*moira.SubscriptionData, 0, 100)
	for user := range usersSet {
		subscriptionIDs, err := database.GetUserSubscriptionIDs(user)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}

		subscriptionsByUser, err := database.GetSubscriptions(subscriptionIDs)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}

		for _, subscriptionByUser := range subscriptionsByUser {
			if subscriptionByUser != nil {
				subscriptions = append(subscriptions, subscriptionByUser)
			}
		}
	}
	result.List = make([]dto.SubscriptionFiltered, 0, len(subscriptions))

	// filter subscriptions and escalations by contact ids
	for _, subscription := range subscriptions {
		matchedEsc := make([]bool, len(subscription.Escalations)) // indicated which escalations have matched
		matchedSub := false                                       // indicates if subscription itself has matched
		matchedAny := false                                       // indicates if anything (either subscription or at least one escalation) has matched

		// define if subscription itself matches any contact
		for _, contactID := range subscription.Contacts {
			if contactID != "" && contactsMatchedSet[contactID] {
				matchedAny = true
				matchedSub = true
				break
			}
		}

		// for each escalation define if it matches any contact
		for i, escalation := range subscription.Escalations {
			for _, contactID := range escalation.Contacts {
				if contactID != "" && contactsMatchedSet[contactID] {
					matchedAny = true
					matchedEsc[i] = true
					break // range escalation.Contacts
				}
			}
		}

		// don't include unmatched subscription into result set
		if !matchedAny {
			continue
		}

		// make a copy of escalations
		escalations := make([]dto.EscalationFiltered, 0, len(subscription.Escalations))
		for _, escalation := range subscription.Escalations {
			escalations = append(escalations, dto.EscalationFiltered{
				ID:              escalation.ID,
				Contacts:        transformContactIDs(contactsAllMap, escalation.Contacts),
				OffsetInMinutes: escalation.OffsetInMinutes,
			})
		}

		// search result
		result.List = append(result.List, dto.SubscriptionFiltered{
			ID:          subscription.ID,
			Enabled:     subscription.Enabled,
			Tags:        subscription.Tags,
			User:        subscription.User,
			Contacts:    transformContactIDs(contactsAllMap, subscription.Contacts),
			Escalations: escalations,
			MatchedEsc:  matchedEsc,
			MatchedSub:  matchedSub,
		})
	}

	return result, nil
}

// GetSubscriptionById is a proxy for the same database method
func GetSubscriptionById(database moira.Database, id string) (*moira.SubscriptionData, *api.ErrorResponse) {
	subscriptionData, err := database.GetSubscription(id)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	} else {
		return &subscriptionData, nil
	}
}

// GetUserSubscriptions get all user subscriptions
func GetUserSubscriptions(database moira.Database, userLogin string) (*dto.SubscriptionList, *api.ErrorResponse) {
	subscriptionIDs, err := database.GetUserSubscriptionIDs(userLogin)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	subscriptions, err := database.GetSubscriptions(subscriptionIDs)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	subscriptionsList := &dto.SubscriptionList{
		List: make([]moira.SubscriptionData, 0),
	}
	for _, subscription := range subscriptions {
		if subscription != nil {
			subscriptionsList.List = append(subscriptionsList.List, *subscription)
		}
	}
	return subscriptionsList, nil
}

// CreateSubscription create or update subscription
func CreateSubscription(dataBase moira.Database, userLogin string, subscription *dto.Subscription) *api.ErrorResponse {
	if subscription.ID == "" {
		subscription.ID = uuid.NewV4().String()
	} else {
		exists, err := isSubscriptionExists(dataBase, subscription.ID)
		if err != nil {
			return api.ErrorInternalServer(err)
		}
		if exists {
			return api.ErrorInvalidRequest(fmt.Errorf("Subscription with this ID already exists"))
		}
	}

	subscription.User = userLogin
	data := moira.SubscriptionData(*subscription)
	if err := dataBase.SaveSubscription(&data); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// UpdateSubscription updates existing subscription
func UpdateSubscription(dataBase moira.Database, subscriptionID string, userLogin string, subscription *dto.Subscription) *api.ErrorResponse {
	subscription.ID = subscriptionID
	subscription.User = userLogin
	data := moira.SubscriptionData(*subscription)
	if err := dataBase.SaveSubscription(&data); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// RemoveSubscription deletes subscription
func RemoveSubscription(database moira.Database, subscriptionID string) *api.ErrorResponse {
	if err := database.RemoveSubscription(subscriptionID); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// SendTestNotification push test notification to verify the correct notification settings
func SendTestNotification(database moira.Database, subscriptionID string) *api.ErrorResponse {
	var value float64 = 1
	eventData := &moira.NotificationEvent{
		SubscriptionID: &subscriptionID,
		Metric:         "Test.metric.value",
		Value:          &value,
		OldState:       moira.TEST,
		State:          moira.TEST,
		Timestamp:      int64(date.DateParamToEpoch("now", "", time.Now().Add(-24*time.Hour).Unix(), time.UTC)),
	}

	if err := database.PushNotificationEvent(eventData); err != nil {
		return api.ErrorInternalServer(err)
	}

	return nil
}

// CheckUserPermissionsForSubscription checks subscription for existence and permissions for given user
func CheckUserPermissionsForSubscription(dataBase moira.Database, subscriptionID string, userLogin string) (moira.SubscriptionData, *api.ErrorResponse) {
	subscription, err := dataBase.GetSubscription(subscriptionID)
	if err != nil {
		if err == database.ErrNil {
			return subscription, api.ErrorNotFound(fmt.Sprintf("Subscription with ID '%s' does not exists", subscriptionID))
		}
		return subscription, api.ErrorInternalServer(err)
	}
	if subscription.User != userLogin {
		return subscription, api.ErrorForbidden("You have not permissions")
	}
	return subscription, nil
}

func isSubscriptionExists(dataBase moira.Database, subscriptionID string) (bool, error) {
	_, err := dataBase.GetSubscription(subscriptionID)
	if err == database.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
