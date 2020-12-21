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
	"go.avito.ru/DO/moira/mock/moira-alert"
)

func TestGetEvents(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	defer mockCtrl.Finish()
	triggerID := uuid.NewV4().String()
	var page int64 = 10
	var size int64 = 100

	Convey("Test has events", t, func() {
		var (
			total int64 = 6000000
		)
		dataBase.EXPECT().GetNotificationEvents(triggerID, page*size, size-1).Return([]*moira.NotificationEvent{{State: moira.NODATA, OldState: moira.OK}, {State: moira.OK, OldState: moira.NODATA}}, nil)
		dataBase.EXPECT().GetNotificationEventCount(triggerID, int64(-1)).Return(total)
		list, err := GetTriggerEvents(dataBase, triggerID, page, size)
		So(err, ShouldBeNil)
		So(list, ShouldResemble, &dto.EventsList{
			List:  []moira.NotificationEvent{{State: moira.NODATA, OldState: moira.OK}, {State: moira.OK, OldState: moira.NODATA}},
			Total: total,
			Size:  size,
			Page:  page,
		})
	})

	Convey("Test no events", t, func() {
		var total int64
		dataBase.EXPECT().GetNotificationEvents(triggerID, page*size, size-1).Return(make([]*moira.NotificationEvent, 0), nil)
		dataBase.EXPECT().GetNotificationEventCount(triggerID, int64(-1)).Return(total)
		list, err := GetTriggerEvents(dataBase, triggerID, page, size)
		So(err, ShouldBeNil)
		So(list, ShouldResemble, &dto.EventsList{
			List:  make([]moira.NotificationEvent, 0),
			Total: total,
			Size:  size,
			Page:  page,
		})
	})

	Convey("Test error", t, func() {
		expected := fmt.Errorf("Oooops! Can not get all contacts")
		dataBase.EXPECT().GetNotificationEvents(triggerID, page*size, size-1).Return(nil, expected)
		list, err := GetTriggerEvents(dataBase, triggerID, page, size)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(list, ShouldBeNil)
	})
}
