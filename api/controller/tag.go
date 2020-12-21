package controller

import (
	"fmt"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
)

// GetAllTagsAndSubscriptions get tags subscriptions and triggerIDs
func GetAllTagsAndSubscriptions(database moira.Database, logger moira.Logger) (*dto.TagsStatistics, *api.ErrorResponse) {
	tags, err := database.GetTagNames()
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	tagStats, err := database.GetTagsStats(tags...)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	result := &dto.TagsStatistics{List: make([]dto.TagStatistics, 0, len(tagStats))}
	for _, stats := range tagStats {
		triggers := stats.Triggers
		if triggers == nil {
			triggers = make([]string, 0)
		}

		subscriptions := stats.Subscriptions
		if subscriptions == nil {
			subscriptions = make([]moira.SubscriptionData, 0)
		}

		result.List = append(result.List, dto.TagStatistics{
			TagName:       stats.Name,
			Triggers:      triggers,
			Subscriptions: subscriptions,
		})
	}

	return result, nil
}

// GetAllTags gets all tag names
func GetAllTags(database moira.Database) (*dto.TagsData, *api.ErrorResponse) { // fake comment
	tagsNames, err := database.GetTagNames()
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	tagsData := &dto.TagsData{
		TagNames: tagsNames,
	}

	return tagsData, nil
}

// RemoveTag deletes tag by name
func RemoveTag(database moira.Database, tagName string) (*dto.MessageResponse, *api.ErrorResponse) {
	triggerIDs, err := database.GetTagTriggerIDs(tagName)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	if len(triggerIDs) > 0 {
		return nil, api.ErrorInvalidRequest(fmt.Errorf("This tag is assigned to %v triggers. Remove tag from triggers first", len(triggerIDs)))
	}
	if err = database.RemoveTag(tagName); err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	return &dto.MessageResponse{Message: "tag deleted"}, nil
}
