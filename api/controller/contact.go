package controller

import (
	"fmt"
	"time"

	"github.com/go-graphite/carbonapi/date"
	"github.com/satori/go.uuid"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/database"
)

// GetAllContacts gets all moira contacts
func GetAllContacts(database moira.Database) (*dto.ContactList, *api.ErrorResponse) {
	contacts, err := database.GetAllContacts()
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	contactsList := dto.ContactList{
		List: contacts,
	}
	return &contactsList, nil
}

// CreateContact creates new notification contact for current user
func CreateContact(dataBase moira.Database, contact *dto.Contact, userLogin string) *api.ErrorResponse {
	contactData := moira.ContactData{
		User:          userLogin,
		Type:          contact.Type,
		Value:         contact.Value,
		FallbackValue: contact.FallbackValue,
	}
	if contact.ID == "" {
		contactData.ID = uuid.NewV4().String()
	} else {
		exists, err := isContactExists(dataBase, contact.ID)
		if err != nil {
			return api.ErrorInternalServer(err)
		}
		if exists {
			return api.ErrorInvalidRequest(fmt.Errorf("Contact with this ID already exists"))
		}
	}
	// AD-12984: do not save special Slack contacts without fallbacks
	if contactData.NeedsFallbackValue() && contactData.FallbackValue == "" {
		return api.ErrorInvalidRequest(fmt.Errorf("Contact needs fallback value but it is not set"))
	}

	if err := dataBase.SaveContact(&contactData); err != nil {
		return api.ErrorInternalServer(err)
	}
	contact.User = userLogin
	contact.ID = contactData.ID
	return nil
}

// UpdateContact updates notification contact for current user
func UpdateContact(dataBase moira.Database, contactDTO dto.Contact, contactData moira.ContactData) (dto.Contact, *api.ErrorResponse) {
	contactData.Type = contactDTO.Type
	contactData.Value = contactDTO.Value
	contactData.FallbackValue = contactDTO.FallbackValue

	if contactData.NeedsFallbackValue() && contactData.FallbackValue == "" {
		return contactDTO, api.ErrorInvalidRequest(fmt.Errorf("Contact needs fallback value but it is not set"))
	}
	if err := dataBase.SaveContact(&contactData); err != nil {
		return contactDTO, api.ErrorInternalServer(err)
	}

	contactDTO.User = contactData.User
	contactDTO.ID = contactData.ID
	return contactDTO, nil
}

// RemoveContact deletes notification contact for current user and remove contactID from all subscriptions
func RemoveContact(database moira.Database, contactID string, userLogin string) *api.ErrorResponse {
	subscriptionIDs, err := database.GetUserSubscriptionIDs(userLogin)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	subscriptions, err := database.GetSubscriptions(subscriptionIDs)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	subscriptionsWithDeletingContact := make([]*moira.SubscriptionData, 0)

	for _, subscription := range subscriptions {
		if subscription == nil {
			continue
		}
		for i, contact := range subscription.Contacts {
			if contact == contactID {
				subscription.Contacts = append(subscription.Contacts[:i], subscription.Contacts[i+1:]...)
				subscriptionsWithDeletingContact = append(subscriptionsWithDeletingContact, subscription)
				break
			}
		}
	}

	if err := database.RemoveContact(contactID); err != nil {
		return api.ErrorInternalServer(err)
	}

	if err := database.SaveSubscriptions(subscriptionsWithDeletingContact); err != nil {
		return api.ErrorInternalServer(err)
	}

	return nil
}

// SendTestContactNotification push test notification to verify the correct contact settings
func SendTestContactNotification(dataBase moira.Database, contactID string) *api.ErrorResponse {
	var value float64 = 1
	eventData := &moira.NotificationEvent{
		ContactID: contactID,
		Metric:    "Test.metric.value",
		Value:     &value,
		OldState:  moira.TEST,
		State:     moira.TEST,
		Timestamp: int64(date.DateParamToEpoch("now", "", time.Now().Add(-24*time.Hour).Unix(), time.UTC)),
	}
	if err := dataBase.PushNotificationEvent(eventData); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// CheckUserPermissionsForContact checks contact for existence and permissions for given user
func CheckUserPermissionsForContact(dataBase moira.Database, contactID string, userLogin string) (moira.ContactData, *api.ErrorResponse) {
	contactData, err := dataBase.GetContact(contactID)
	if err != nil {
		if err == database.ErrNil {
			return contactData, api.ErrorNotFound(fmt.Sprintf("Contact with ID '%s' does not exists", contactID))
		}
		return contactData, api.ErrorInternalServer(err)
	}
	if contactData.User != userLogin {
		return contactData, api.ErrorForbidden("You have not permissions")
	}
	return contactData, nil
}

func isContactExists(dataBase moira.Database, contactID string) (bool, error) {
	_, err := dataBase.GetContact(contactID)
	if err == database.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// transformContactIDs transforms contact ids list to contact data list
func transformContactIDs(contactsAll map[string]moira.ContactData, contactIDs []string) []moira.ContactData {
	result := make([]moira.ContactData, 0, len(contactIDs))
	for _, contactID := range contactIDs {
		if contactData, ok := contactsAll[contactID]; ok {
			result = append(result, contactData)
		}
	}
	return result
}
