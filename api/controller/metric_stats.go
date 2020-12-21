package controller

import (
	"sort"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
)

func GetMetricStats(database moira.Database, intervalLength int64, onlyErrors bool, filterTags []string) (*dto.MetricStats, *api.ErrorResponse) {
	var (
		intervalEnd   = time.Now().Unix()
		intervalStart = intervalEnd - intervalLength
	)
	allEvents, err := database.GetAllNotificationEvents(intervalStart, intervalEnd)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	triggerIDList, err := database.GetTriggerCheckIDs(filterTags, onlyErrors)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	// get trigger data
	triggerDataList, err := database.GetTriggers(triggerIDList)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	// make a Trigger ID-to-data map
	triggerData := make(map[string]*moira.Trigger, len(triggerDataList))
	for _, trigger := range triggerDataList {
		triggerData[trigger.ID] = trigger
	}

	type metricKey struct {
		Metric    string
		TriggerID string
	}
	errorCounts := make(map[metricKey]int64)
	currentStates := make(map[metricKey]string)
	for _, event := range allEvents {
		if event.IsTriggerEvent {
			continue
		}
		if _, found := triggerData[event.TriggerID]; found {
			// this trigger does have the tags we want
			key := metricKey{Metric: event.Metric, TriggerID: event.TriggerID}
			if event.OldState == moira.OK && event.State != moira.OK {
				// this is a flap, add it to the stats
				errorCounts[key] += 1
			}
			currentStates[key] = event.State
		}
	}

	result := dto.MetricStats{
		List: make([]*dto.MetricStatModel, 0, len(errorCounts)),
	}
	for key, errorCount := range errorCounts {
		if onlyErrors && currentStates[key] == moira.OK {
			continue
		}
		stat := dto.MetricStatModel{
			Metric:       key.Metric,
			Trigger:      dto.CreateTriggerModel(triggerData[key.TriggerID]),
			ErrorCount:   errorCount,
			CurrentState: currentStates[key],
		}
		result.List = append(result.List, &stat)
	}
	sort.SliceStable(result.List, func(i, j int) bool {
		return result.List[i].ErrorCount > result.List[j].ErrorCount
	})
	return &result, nil
}
