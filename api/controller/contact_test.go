package controller

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/satori/go.uuid"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/mock/moira-alert"
)

func TestGetAllContacts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)

	Convey("Error get all contacts", t, func() {
		expected := fmt.Errorf("Oooops! Can not get all contacts")
		dataBase.EXPECT().GetAllContacts().Return(nil, expected)
		contacts, err := GetAllContacts(dataBase)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(contacts, ShouldBeNil)
	})

	Convey("Get contacts", t, func() {
		contacts := []*moira.ContactData{
			{
				ID:    uuid.NewV4().String(),
				Type:  "mail",
				User:  "user1",
				Value: "good@mail.com",
			},
			{
				ID:    uuid.NewV4().String(),
				Type:  "pushover",
				User:  "user2",
				Value: "ggg1",
			},
		}
		dataBase.EXPECT().GetAllContacts().Return(contacts, nil)
		actual, err := GetAllContacts(dataBase)
		So(err, ShouldBeNil)
		So(actual, ShouldResemble, &dto.ContactList{List: contacts})
	})

	Convey("No contacts", t, func() {
		dataBase.EXPECT().GetAllContacts().Return(make([]*moira.ContactData, 0), nil)
		contacts, err := GetAllContacts(dataBase)
		So(err, ShouldBeNil)
		So(contacts, ShouldResemble, &dto.ContactList{List: make([]*moira.ContactData, 0)})
	})
}

func TestCreateContact(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	defer mockCtrl.Finish()
	userLogin := "user"

	Convey("Success create", t, func() {
		contact := &dto.Contact{
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		dataBase.EXPECT().SaveContact(gomock.Any()).Return(nil)
		err := CreateContact(dataBase, contact, userLogin)
		So(err, ShouldBeNil)
		So(contact.User, ShouldResemble, userLogin)
	})

	Convey("Success create contact with id", t, func() {
		contact := &dto.Contact{
			ID:            uuid.NewV4().String(),
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		dataBase.EXPECT().GetContact(contact.ID).Return(moira.ContactData{}, database.ErrNil)
		dataBase.EXPECT().SaveContact(gomock.Any()).Return(nil)
		err := CreateContact(dataBase, contact, userLogin)
		So(err, ShouldBeNil)
		So(contact.User, ShouldResemble, userLogin)
		So(contact.ID, ShouldResemble, contact.ID)
	})

	Convey("Contact exists by id", t, func() {
		contact := &dto.Contact{
			ID:            uuid.NewV4().String(),
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		dataBase.EXPECT().GetContact(contact.ID).Return(moira.ContactData{}, nil)
		err := CreateContact(dataBase, contact, userLogin)
		So(err, ShouldResemble, api.ErrorInvalidRequest(fmt.Errorf("Contact with this ID already exists")))
	})

	Convey("Error get contact", t, func() {
		contact := &dto.Contact{
			ID:            uuid.NewV4().String(),
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		err := fmt.Errorf("Oooops! Can not write contact")
		dataBase.EXPECT().GetContact(contact.ID).Return(moira.ContactData{}, err)
		expected := CreateContact(dataBase, contact, userLogin)
		So(expected, ShouldResemble, api.ErrorInternalServer(err))
	})

	Convey("Error save contact", t, func() {
		contact := &dto.Contact{
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		err := fmt.Errorf("Oooops! Can not write contact")
		dataBase.EXPECT().SaveContact(gomock.Any()).Return(err)
		expected := CreateContact(dataBase, contact, userLogin)
		So(expected, ShouldResemble, &api.ErrorResponse{
			ErrorText:      err.Error(),
			HTTPStatusCode: 500,
			StatusText:     "Internal Server Error",
			Err:            err,
		})
	})

	Convey("Error save special Slack contact without fallback", t, func() {
		contactDTO := dto.Contact{
			Value: "_deployer",
			Type:  "slack",
		}
		contactID := uuid.NewV4().String()
		contact := moira.ContactData{
			Value: contactDTO.Value,
			Type:  contactDTO.Type,
			ID:    contactID,
			User:  userLogin,
		}
		err := fmt.Errorf("Contact needs fallback value but it is not set")
		exprectedContact, actual := UpdateContact(dataBase, contactDTO, contact)
		So(actual, ShouldResemble, api.ErrorInvalidRequest(err))
		So(exprectedContact.User, ShouldResemble, contactDTO.User)
		So(exprectedContact.ID, ShouldResemble, contactDTO.ID)
	})
}

func TestUpdateContact(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	defer mockCtrl.Finish()
	userLogin := "user"

	Convey("Success update", t, func() {
		contactDTO := dto.Contact{
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		contactID := uuid.NewV4().String()
		contact := moira.ContactData{
			Value:         contactDTO.Value,
			Type:          contactDTO.Type,
			ID:            contactID,
			User:          userLogin,
			FallbackValue: contactDTO.FallbackValue,
		}
		dataBase.EXPECT().SaveContact(&contact).Return(nil)
		expectedContact, err := UpdateContact(dataBase, contactDTO, moira.ContactData{
			ID:            contactID,
			User:          userLogin,
			FallbackValue: contactDTO.FallbackValue,
		})
		So(err, ShouldBeNil)
		So(expectedContact.User, ShouldResemble, userLogin)
		So(expectedContact.ID, ShouldResemble, contactID)
	})

	Convey("Error save", t, func() {
		contactDTO := dto.Contact{
			Value:         "some@mail.com",
			Type:          "mail",
			FallbackValue: "fallback@mail.org",
		}
		contactID := uuid.NewV4().String()
		contact := moira.ContactData{
			Value:         contactDTO.Value,
			Type:          contactDTO.Type,
			ID:            contactID,
			User:          userLogin,
			FallbackValue: contactDTO.FallbackValue,
		}
		err := fmt.Errorf("Oooops")
		dataBase.EXPECT().SaveContact(&contact).Return(err)
		exprectedContact, actual := UpdateContact(dataBase, contactDTO, contact)
		So(actual, ShouldResemble, api.ErrorInternalServer(err))
		So(exprectedContact.User, ShouldResemble, contactDTO.User)
		So(exprectedContact.ID, ShouldResemble, contactDTO.ID)
	})

	Convey("Error save special Slack contact without fallback", t, func() {
		contactDTO := dto.Contact{
			Value: "_deployer",
			Type:  "slack",
		}
		contactID := uuid.NewV4().String()
		contact := moira.ContactData{
			Value: contactDTO.Value,
			Type:  contactDTO.Type,
			ID:    contactID,
			User:  userLogin,
		}
		err := fmt.Errorf("Contact needs fallback value but it is not set")
		exprectedContact, actual := UpdateContact(dataBase, contactDTO, contact)
		So(actual, ShouldResemble, api.ErrorInvalidRequest(err))
		So(exprectedContact.User, ShouldResemble, contactDTO.User)
		So(exprectedContact.ID, ShouldResemble, contactDTO.ID)
	})
}

func TestRemoveContact(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	userLogin := "user"
	contactID := uuid.NewV4().String()

	Convey("Delete contact without user subscriptions", t, func() {
		dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return(make([]string, 0), nil)
		dataBase.EXPECT().GetSubscriptions(make([]string, 0)).Return(make([]*moira.SubscriptionData, 0), nil)
		dataBase.EXPECT().RemoveContact(contactID).Return(nil)
		dataBase.EXPECT().SaveSubscriptions(make([]*moira.SubscriptionData, 0)).Return(nil)
		err := RemoveContact(dataBase, contactID, userLogin)
		So(err, ShouldBeNil)
	})

	Convey("Delete contact without contact subscriptions", t, func() {
		subscription := &moira.SubscriptionData{
			Contacts: []string{uuid.NewV4().String()},
			ID:       uuid.NewV4().String(),
		}

		dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return([]string{subscription.ID}, nil)
		dataBase.EXPECT().GetSubscriptions([]string{subscription.ID}).Return([]*moira.SubscriptionData{subscription}, nil)
		dataBase.EXPECT().RemoveContact(contactID).Return(nil)
		dataBase.EXPECT().SaveSubscriptions(make([]*moira.SubscriptionData, 0)).Return(nil)
		err := RemoveContact(dataBase, contactID, userLogin)
		So(err, ShouldBeNil)
	})

	Convey("Delete contact with contact subscriptions", t, func() {
		subscription := moira.SubscriptionData{
			Contacts: []string{contactID},
			ID:       uuid.NewV4().String(),
		}
		expectedSub := subscription
		expectedSub.Contacts = make([]string, 0)

		dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return([]string{subscription.ID}, nil)
		dataBase.EXPECT().GetSubscriptions([]string{subscription.ID}).Return([]*moira.SubscriptionData{&subscription}, nil)
		dataBase.EXPECT().RemoveContact(contactID).Return(nil)
		dataBase.EXPECT().SaveSubscriptions([]*moira.SubscriptionData{&expectedSub}).Return(nil)
		err := RemoveContact(dataBase, contactID, userLogin)
		So(err, ShouldBeNil)
	})

	Convey("Error tests", t, func() {
		Convey("GetUserSubscriptionIDs", func() {
			expectedError := fmt.Errorf("Oooops! Can not read user subscription ids")
			dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return(nil, expectedError)
			err := RemoveContact(dataBase, contactID, userLogin)
			So(err, ShouldResemble, api.ErrorInternalServer(expectedError))
		})
		Convey("GetSubscriptions", func() {
			expectedError := fmt.Errorf("Oooops! Can not read user subscriptions")
			dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return(make([]string, 0), nil)
			dataBase.EXPECT().GetSubscriptions(make([]string, 0)).Return(nil, expectedError)
			err := RemoveContact(dataBase, contactID, userLogin)
			So(err, ShouldResemble, api.ErrorInternalServer(expectedError))
		})
		Convey("RemoveContact", func() {
			expectedError := fmt.Errorf("Oooops! Can not delete contact")
			dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return(make([]string, 0), nil)
			dataBase.EXPECT().GetSubscriptions(make([]string, 0)).Return(make([]*moira.SubscriptionData, 0), nil)
			dataBase.EXPECT().RemoveContact(contactID).Return(expectedError)
			err := RemoveContact(dataBase, contactID, userLogin)
			So(err, ShouldResemble, api.ErrorInternalServer(expectedError))
		})
		Convey("SaveSubscriptions", func() {
			expectedError := fmt.Errorf("Oooops! Can not write subscriptions")
			dataBase.EXPECT().GetUserSubscriptionIDs(userLogin).Return(make([]string, 0), nil)
			dataBase.EXPECT().GetSubscriptions(make([]string, 0)).Return(make([]*moira.SubscriptionData, 0), nil)
			dataBase.EXPECT().RemoveContact(contactID).Return(nil)
			dataBase.EXPECT().SaveSubscriptions(make([]*moira.SubscriptionData, 0)).Return(expectedError)
			err := RemoveContact(dataBase, contactID, userLogin)
			So(err, ShouldResemble, api.ErrorInternalServer(expectedError))
		})
	})
}

func TestSendTestContactNotification(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	id := uuid.NewV4().String()

	Convey("Success", t, func() {
		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(nil)
		err := SendTestContactNotification(dataBase, id)
		So(err, ShouldBeNil)
	})

	Convey("Error", t, func() {
		expected := fmt.Errorf("Oooops! Can not push event")
		dataBase.EXPECT().PushNotificationEvent(gomock.Any()).Return(expected)
		err := SendTestContactNotification(dataBase, id)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
	})
}

func TestCheckUserPermissionsForContact(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	userLogin := uuid.NewV4().String()
	id := uuid.NewV4().String()

	Convey("No contact", t, func() {
		dataBase.EXPECT().GetContact(id).Return(moira.ContactData{}, database.ErrNil)
		expectedContact, expected := CheckUserPermissionsForContact(dataBase, id, userLogin)
		So(expected, ShouldResemble, api.ErrorNotFound(fmt.Sprintf("Contact with ID '%s' does not exists", id)))
		So(expectedContact, ShouldResemble, moira.ContactData{})
	})

	Convey("Different user", t, func() {
		dataBase.EXPECT().GetContact(id).Return(moira.ContactData{User: "diffUser"}, nil)
		expectedContact, expected := CheckUserPermissionsForContact(dataBase, id, userLogin)
		So(expected, ShouldResemble, api.ErrorForbidden("You have not permissions"))
		So(expectedContact, ShouldResemble, moira.ContactData{User: "diffUser"})
	})

	Convey("Has contact", t, func() {
		actualContact := moira.ContactData{ID: id, User: userLogin}
		dataBase.EXPECT().GetContact(id).Return(actualContact, nil)
		expectedContact, expected := CheckUserPermissionsForContact(dataBase, id, userLogin)
		So(expected, ShouldBeNil)
		So(expectedContact, ShouldResemble, actualContact)
	})

	Convey("Error get contact", t, func() {
		err := fmt.Errorf("Oooops! Can not read contact")
		dataBase.EXPECT().GetContact(id).Return(moira.ContactData{User: userLogin}, err)
		expectedContact, expected := CheckUserPermissionsForContact(dataBase, id, userLogin)
		So(expected, ShouldResemble, api.ErrorInternalServer(err))
		So(expectedContact, ShouldResemble, moira.ContactData{User: userLogin})
	})
}
