package controller

import (
	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
)

// GetTriggerEvents gets trigger event from current page and all trigger event count
func GetTriggerEvents(database moira.Database, triggerID string, page int64, size int64) (*dto.EventsList, *api.ErrorResponse) {
	events, err := database.GetNotificationEvents(triggerID, page*size, size-1)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	eventCount := database.GetNotificationEventCount(triggerID, -1)

	eventsList := &dto.EventsList{
		Size:  size,
		Page:  page,
		Total: eventCount,
		List:  make([]moira.NotificationEvent, 0, size),
	}

	for _, event := range events {
		if event != nil {
			eventsList.List = append(eventsList.List, *event)
		}
	}

	return eventsList, nil
}
