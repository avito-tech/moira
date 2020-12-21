package controller

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestGetAllTags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	database := mock_moira_alert.NewMockDatabase(mockCtrl)

	Convey("Success", t, func() {
		database.EXPECT().GetTagNames().Return([]string{"tag21", "tag22", "tag1"}, nil)
		data, err := GetAllTags(database)
		So(err, ShouldBeNil)
		So(data, ShouldResemble, &dto.TagsData{TagNames: []string{"tag21", "tag22", "tag1"}})
	})

	Convey("Error", t, func() {
		expected := fmt.Errorf("Nooooooooooooooooooooo")
		database.EXPECT().GetTagNames().Return(nil, expected)
		data, err := GetAllTags(database)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(data, ShouldBeNil)
	})
}

func TestDeleteTag(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	database := mock_moira_alert.NewMockDatabase(mockCtrl)
	tag := "MyTag"

	Convey("Test no trigger ids by tag", t, func() {
		database.EXPECT().GetTagTriggerIDs(tag).Return(nil, nil)
		database.EXPECT().RemoveTag(tag).Return(nil)
		resp, err := RemoveTag(database, tag)
		So(err, ShouldBeNil)
		So(resp, ShouldResemble, &dto.MessageResponse{Message: "tag deleted"})
	})

	Convey("Test has trigger ids by tag", t, func() {
		database.EXPECT().GetTagTriggerIDs(tag).Return([]string{"123"}, nil)
		resp, err := RemoveTag(database, tag)
		So(err, ShouldResemble, api.ErrorInvalidRequest(fmt.Errorf("This tag is assigned to %v triggers. Remove tag from triggers first", 1)))
		So(resp, ShouldBeNil)
	})

	Convey("GetTagTriggerIDs error", t, func() {
		expected := fmt.Errorf("Can not read trigger ids")
		database.EXPECT().GetTagTriggerIDs(tag).Return(nil, expected)
		resp, err := RemoveTag(database, tag)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(resp, ShouldBeNil)
	})

	Convey("Error delete tag", t, func() {
		expected := fmt.Errorf("Can not delete tag")
		database.EXPECT().GetTagTriggerIDs(tag).Return(nil, nil)
		database.EXPECT().RemoveTag(tag).Return(expected)
		resp, err := RemoveTag(database, tag)
		So(err, ShouldResemble, api.ErrorInternalServer(expected))
		So(resp, ShouldBeNil)
	})
}

func TestGetAllTagsAndSubscriptions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	database := mock_moira_alert.NewMockDatabase(mockCtrl)
	logger := test_helpers.GetTestLogger()

	Convey("Success get tag stats", t, func() {
		tags := []string{"tag21", "tag22", "tag1"}
		database.EXPECT().GetTagNames().Return(tags, nil)
		database.EXPECT().GetTagsStats(tags).Return([]moira.TagStats{
			{
				Name:          "tag1",
				Triggers:      make([]string, 0),
				Subscriptions: []moira.SubscriptionData{{Tags: []string{"tag1", "tag2"}}},
			},
			{
				Name:          "tag21",
				Triggers:      []string{"trigger21"},
				Subscriptions: []moira.SubscriptionData{{Tags: []string{"tag21"}}},
			},
			{
				Name:          "tag22",
				Triggers:      []string{"trigger22"},
				Subscriptions: make([]moira.SubscriptionData, 0),
			},
		}, nil)
		stat, err := GetAllTagsAndSubscriptions(database, logger)
		So(err, ShouldBeNil)
		So(stat.List, ShouldHaveLength, 3)
		for _, stat := range stat.List {
			if stat.TagName == "tag21" {
				So(stat, ShouldResemble, dto.TagStatistics{TagName: "tag21", Triggers: []string{"trigger21"}, Subscriptions: []moira.SubscriptionData{{Tags: []string{"tag21"}}}})
			}
			if stat.TagName == "tag22" {
				So(stat, ShouldResemble, dto.TagStatistics{TagName: "tag22", Triggers: []string{"trigger22"}, Subscriptions: make([]moira.SubscriptionData, 0)})
			}
			if stat.TagName == "tag1" {
				So(stat, ShouldResemble, dto.TagStatistics{TagName: "tag1", Triggers: make([]string, 0), Subscriptions: []moira.SubscriptionData{{Tags: []string{"tag1", "tag2"}}}})
			}
		}
	})

	Convey("Errors", t, func() {
		Convey("GetTagNames", func() {
			expected := fmt.Errorf("Can not get tag names")
			tags := []string{"tag21", "tag22", "tag1"}
			database.EXPECT().GetTagNames().Return(tags, expected)
			stat, err := GetAllTagsAndSubscriptions(database, logger)
			So(err, ShouldResemble, api.ErrorInternalServer(expected))
			So(stat, ShouldBeNil)
		})
	})
}
