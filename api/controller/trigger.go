package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/checker"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/target"
)

// UpdateTrigger update trigger data and trigger metrics in last state
func UpdateTrigger(
	dataBase moira.Database,
	triggerInheritanceDatabase moira.TriggerInheritanceDatabase,
	trigger *dto.TriggerModel,
	triggerID string,
	timeSeriesNames map[string]bool,
) (*dto.SaveTriggerResponse, *api.ErrorResponse) {
	prevTrigger, err := dataBase.GetTrigger(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return nil, api.ErrorNotFound(fmt.Sprintf("Trigger with ID = '%s' does not exists", triggerID))
		}
		return nil, api.ErrorInternalServer(err)
	}

	logging.GetLogger(triggerID).InfoE("About to call saveTrigger (update)", map[string]interface{}{
		"prev": prevTrigger,
		"curr": trigger,
	})
	return saveTrigger(dataBase, triggerInheritanceDatabase, trigger.ToMoiraTrigger(), triggerID, timeSeriesNames)
}

// saveTrigger create or update trigger data and update trigger metrics in last state
func saveTrigger(
	db moira.Database,
	triggerInheritanceDatabase moira.TriggerInheritanceDatabase,
	trigger *moira.Trigger,
	triggerID string,
	timeSeriesNames map[string]bool,
) (*dto.SaveTriggerResponse, *api.ErrorResponse) {
	if err := db.AcquireTriggerCheckLock(triggerID); err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	defer db.DeleteTriggerCheckLock(triggerID)

	lastCheck, err := db.GetOrCreateTriggerLastCheck(triggerID)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	for metric := range lastCheck.Metrics {
		if _, ok := timeSeriesNames[metric]; !ok {
			delete(lastCheck.Metrics, metric)
		}
	}
	lastCheck.UpdateScore()

	// save states
	if err = db.SaveTrigger(triggerID, trigger); err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	if err = db.SetTriggerLastCheck(triggerID, lastCheck); err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	if triggerInheritanceDatabase != nil {
		// TODO mock triggerInheritanceDatabase for unit tests
		// TODO: validation
		if err = triggerInheritanceDatabase.SetTriggerParents(triggerID, trigger.Parents); err != nil {
			return nil, api.ErrorInternalServer(err)
		}

		if err = db.UpdateInheritanceDataVersion(); err != nil {
			return nil, api.ErrorInternalServer(err)
		}
	}

	return &dto.SaveTriggerResponse{
		ID:      triggerID,
		Message: "trigger updated",
	}, nil
}

// GetTrigger gets trigger with his throttling - next allowed message time
func GetTrigger(dataBase moira.Database, triggerID string) (*dto.Trigger, *api.ErrorResponse) {
	trigger, err := dataBase.GetTrigger(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return nil, api.ErrorNotFound("Trigger not found")
		}
		return nil, api.ErrorInternalServer(err)
	}
	throttling, _ := dataBase.GetTriggerThrottling(triggerID)
	throttlingUnix := throttling.Unix()

	if throttlingUnix < time.Now().Unix() {
		throttlingUnix = 0
	}

	// for web-version it only matters if trigger has escalations not resolutions
	hasEscalations, err := dataBase.TriggerHasPendingEscalations(triggerID, false)
	if err != nil {
		hasEscalations = false
	}

	parentTriggersDB, err := dataBase.GetTriggers(trigger.Parents)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	parentTriggers := make([]*dto.Trigger, len(trigger.Parents)) // [sic]
	for i, parentTrigger := range parentTriggersDB {
		if parentTrigger != nil {
			parentTriggers[i] = &dto.Trigger{TriggerModel: dto.CreateTriggerModel(parentTrigger)}
		} else {
			parentTriggers[i] = nil
		}
	}

	triggerResponse := dto.Trigger{
		TriggerModel:   dto.CreateTriggerModel(trigger),
		Throttling:     throttlingUnix,
		HasEscalations: hasEscalations,
		ParentTriggers: parentTriggers,
	}
	return &triggerResponse, nil
}

// RemoveTrigger deletes trigger by given triggerID
func RemoveTrigger(database moira.Database, triggerID string) *api.ErrorResponse {
	prevTrigger, err := database.GetTrigger(triggerID)
	logging.GetLogger(triggerID).InfoE("About to remove trigger", map[string]interface{}{
		"prev": prevTrigger,
		"err":  err,
	})

	if err := database.RemoveTrigger(triggerID); err != nil {
		return api.ErrorInternalServer(err)
	}
	if err := database.RemoveTriggerLastCheck(triggerID); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// GetTriggerThrottling gets trigger throttling timestamp
func GetTriggerThrottling(database moira.Database, triggerID string) (*dto.ThrottlingResponse, *api.ErrorResponse) {
	throttling, _ := database.GetTriggerThrottling(triggerID)
	throttlingUnix := throttling.Unix()
	if throttlingUnix < time.Now().Unix() {
		throttlingUnix = 0
	}
	return &dto.ThrottlingResponse{Throttling: throttlingUnix}, nil
}

// GetTriggerLastCheck gets trigger last check data
func GetTriggerLastCheck(dataBase moira.Database, triggerID string) (*dto.TriggerCheck, *api.ErrorResponse) {
	lastCheck, err := dataBase.GetTriggerLastCheck(triggerID)
	if err != nil {
		if err != database.ErrNil {
			return nil, api.ErrorInternalServer(err)
		}
		lastCheck = nil
	}

	maintenance, err := dataBase.GetMaintenanceTrigger(triggerID)
	if err != nil && err != database.ErrNil {
		return nil, api.ErrorInternalServer(err)
	}

	triggerCheck := dto.TriggerCheck{
		CheckData:   lastCheck,
		TriggerID:   triggerID,
		Maintenance: maintenance,
	}

	return &triggerCheck, nil
}

// DeleteTriggerThrottling deletes trigger throttling
func DeleteTriggerThrottling(database moira.Database, triggerID string) *api.ErrorResponse {
	if err := database.DeleteTriggerThrottling(triggerID); err != nil {
		return api.ErrorInternalServer(err)
	}

	now := time.Now().Unix()
	notifications, _, err := database.GetNotifications(0, -1)
	if err != nil {
		return api.ErrorInternalServer(err)
	}
	notificationsForRewrite := make([]*moira.ScheduledNotification, 0)
	for _, notification := range notifications {
		if notification != nil && notification.Event.TriggerID == triggerID {
			notificationsForRewrite = append(notificationsForRewrite, notification)
		}
	}
	if err = database.AddNotifications(notificationsForRewrite, now); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// DeleteTriggerMetric deletes metric from last check and all trigger patterns metrics
func DeleteTriggerMetric(dataBase moira.Database, metricName string, triggerID string) *api.ErrorResponse {
	trigger, err := dataBase.GetTrigger(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return api.ErrorInvalidRequest(fmt.Errorf("Trigger not found"))
		}
		return api.ErrorInternalServer(err)
	}

	if err = dataBase.AcquireTriggerCheckLock(triggerID); err != nil {
		return api.ErrorInternalServer(err)
	}
	defer dataBase.DeleteTriggerCheckLock(triggerID)

	lastCheck, err := dataBase.GetTriggerLastCheck(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return api.ErrorInvalidRequest(fmt.Errorf("Trigger check not found"))
		}
		return api.ErrorInternalServer(err)
	}
	_, ok := lastCheck.Metrics[metricName]
	if ok {
		delete(lastCheck.Metrics, metricName)
		lastCheck.UpdateScore()
	}
	if err = dataBase.RemovePatternsMetrics(trigger.Patterns); err != nil {
		return api.ErrorInternalServer(err)
	}
	if err = dataBase.SetTriggerLastCheck(triggerID, lastCheck); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// SetMetricsMaintenance sets maintenance for some metrics in a trigger
func SetMetricsMaintenance(database moira.Database, triggerID string, update dto.MetricsMaintenance) *api.ErrorResponse {
	if err := database.AcquireTriggerMaintenanceLock(triggerID); err != nil {
		return api.ErrorInternalServer(err)
	}
	defer database.DeleteTriggerMaintenanceLock(triggerID)

	maintenance, err := database.GetMaintenanceTrigger(triggerID)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	for metric, until := range update {
		if until == 0 {
			maintenance.Del(metric)
		} else {
			maintenance.Add(metric, until)
		}
		maintenance.Add(metric, until)
	}

	if err = database.SetMaintenanceTrigger(triggerID, maintenance); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// SetTriggerMaintenance sets maintenance for the entire trigger
func SetTriggerMaintenance(database moira.Database, triggerID string, until int64) *api.ErrorResponse {
	if err := database.AcquireTriggerMaintenanceLock(triggerID); err != nil {
		return api.ErrorInternalServer(err)
	}
	defer database.DeleteTriggerMaintenanceLock(triggerID)

	maintenance, err := database.GetMaintenanceTrigger(triggerID)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	if until == 0 {
		maintenance.Del(moira.WildcardMetric)
	} else {
		maintenance.Add(moira.WildcardMetric, until)
	}

	if err = database.SetMaintenanceTrigger(triggerID, maintenance); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// GetTriggerMetrics gets all trigger metrics values, default values from: now - 10min, to: now
func GetTriggerMetrics(dataBase moira.Database, from, to int64, triggerID string) (dto.TriggerMetrics, *api.ErrorResponse) {
	trigger, err := dataBase.GetTrigger(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return nil, api.ErrorInvalidRequest(fmt.Errorf("Trigger not found"))
		}
		return nil, api.ErrorInternalServer(err)
	}

	triggerMetrics := make(map[string][]moira.MetricValue)
	isSimpleTrigger := trigger.IsSimple()
	for _, tar := range trigger.Targets {
		result, err := target.EvaluateTarget(dataBase, tar, from, to, isSimpleTrigger)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}
		for _, timeSeries := range result.TimeSeries {
			values := make([]moira.MetricValue, 0)
			for i := 0; i < len(timeSeries.Values); i++ {
				timestamp := int64(timeSeries.StartTime + int32(i)*timeSeries.StepTime)
				value := timeSeries.GetTimestampValue(timestamp)
				if !checker.IsInvalidValue(value) {
					values = append(values, moira.MetricValue{Value: value, Timestamp: timestamp})
				}
			}
			triggerMetrics[timeSeries.Name] = values
		}
	}
	return triggerMetrics, nil
}

func AckEscalations(database moira.Database, triggerID string) *api.ErrorResponse {
	lastCheck, err := database.GetTriggerLastCheck(triggerID)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	metrics := make([]string, 0, len(lastCheck.Metrics))
	for metric := range lastCheck.Metrics {
		metrics = append(metrics, metric)
	}

	if err := database.AckEscalationsBatch(triggerID, metrics, false); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

func AckEscalationsMetrics(database moira.Database, triggerID string, metrics []string) *api.ErrorResponse {
	for _, metric := range metrics {
		if err := database.AckEscalations(triggerID, metric, false); err != nil {
			return api.ErrorInternalServer(err)
		}
		if err := database.AckUnacknowledgedMessages(triggerID, metric); err != nil {
			return api.ErrorInternalServer(err)
		}
	}
	return nil
}

func GetUnacknowledgedMessages(database moira.Database, triggerID string, metrics []string) (dto.UnacknowledgedMessages, *api.ErrorResponse) {
	result := make(dto.UnacknowledgedMessages, 0)
	for _, metric := range metrics {
		messages, err := database.GetUnacknowledgedMessages(triggerID, metric)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}
		for _, message := range messages {
			jsonLink, err := json.Marshal(message)
			if err != nil {
				return nil, api.ErrorInternalServer(err)
			}
			result = append(result, dto.UnacknowledgedMessage{
				Sender:      message.SenderName(),
				MessageLink: jsonLink,
			})
		}
	}
	return result, nil
}
