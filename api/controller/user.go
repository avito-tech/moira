package controller

import (
	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
)

// GetUserSettings gets user contacts and subscriptions
func GetUserSettings(database moira.Database, userLogin string, superUsers map[string]bool) (*dto.UserSettings, *api.ErrorResponse) {
	userSettings := &dto.UserSettings{
		User: dto.User{
			Login:       userLogin,
			IsSuperUser: superUsers[userLogin],
		},
		Contacts:      make([]moira.ContactData, 0),
		Subscriptions: make([]moira.SubscriptionData, 0),
	}

	subscriptionIDs, err := database.GetUserSubscriptionIDs(userLogin)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	subscriptions, err := database.GetSubscriptions(subscriptionIDs)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	for _, subscription := range subscriptions {
		if subscription != nil {
			userSettings.Subscriptions = append(userSettings.Subscriptions, *subscription)
		}
	}
	contactIDs, err := database.GetUserContactIDs(userLogin)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	contacts, err := database.GetContacts(contactIDs)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	for _, contact := range contacts {
		if contact != nil {
			userSettings.Contacts = append(userSettings.Contacts, *contact)
		}
	}
	return userSettings, nil
}
