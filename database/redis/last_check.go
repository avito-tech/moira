package redis

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/database/redis/reply"
)

// GetTriggerLastCheck gets trigger last check data by given triggerID, if no value, return database.ErrNil error
func (connector *DbConnector) GetTriggerLastCheck(triggerID string) (*moira.CheckData, error) {
	c := connector.pool.Get()
	defer c.Close()

	return reply.Check(c.Do("GET", metricLastCheckKey(triggerID)))
}

// GetTriggerLastChecks gets last checks for multiple triggers at once
func (connector *DbConnector) GetTriggerLastChecks(triggerIDs []string) (map[string]*moira.CheckData, error) {
	c := connector.pool.Get()
	defer c.Close()

	result := make(map[string]*moira.CheckData, len(triggerIDs))
	redisKeys := convertArgs(triggerIDs, metricLastCheckKey)

	response, err := redis.Values(c.Do("MGET", redisKeys...))
	if err != nil {
		return result, fmt.Errorf("Could not batch get trigger last checks from Redis: %s", err.Error())
	}

	for i, row := range response {
		triggerID := triggerIDs[i]
		checkData, err := reply.Check(row, nil)
		switch err {
		case nil:
			result[triggerID] = checkData
		case database.ErrNil:
			// ignore
		default:
			return result, fmt.Errorf(
				"Could not get trigger [%s] last check from Redis: %s",
				triggerID, err.Error(),
			)
		}
	}

	return result, nil
}

// GetOrCreateTriggerLastCheck gets trigger last check data by given triggerID
// If no value, a new empty one is created (but not saved)
func (connector *DbConnector) GetOrCreateTriggerLastCheck(triggerID string) (*moira.CheckData, error) {
	c := connector.pool.Get()
	defer c.Close()

	checkData, err := connector.GetTriggerLastCheck(triggerID)
	if err == database.ErrNil {
		checkData = &moira.CheckData{
			Metrics: make(map[string]*moira.MetricState),
			State:   moira.NODATA,
		}
		err = nil
	}

	if checkData != nil && checkData.MaintenanceMetric == nil {
		checkData.MaintenanceMetric = make(map[string]int64)
	}

	return checkData, err
}

// SetTriggerLastCheck sets trigger last check data
func (connector *DbConnector) SetTriggerLastCheck(triggerID string, checkData *moira.CheckData) error {
	// ignore check data if trigger was deleted earlier
	triggerExists, err := connector.CheckTriggerExists(triggerID)
	if err != nil {
		return fmt.Errorf("Failed to check if trigger id %s exists: %s", triggerID, err.Error())
	}
	if !triggerExists {
		_ = connector.RemoveTriggerLastCheck(triggerID)
		return fmt.Errorf("Attempt to save trigger id %s that has already been deleted", triggerID)
	}

	bytes, err := json.Marshal(checkData)
	if err != nil {
		return err
	}

	c := connector.pool.Get()
	defer c.Close()
	_ = c.Send("MULTI")

	// auto-migrate maintenance structure
	if checkData.Version == 0 {
		maintenance := moira.NewMaintenanceFromCheckData(checkData)
		if err = connector.setMaintenanceTrigger(c, triggerID, maintenance, false); err != nil {
			return errors.Wrapf(err, "Failed to set maintenance for trigger id %s", triggerID)
		}

		// need to re-marshal because Version has changed
		checkData.Version = 1
		bytes, _ = json.Marshal(checkData)
	}

	// store trigger check data
	c.Send("SET", metricLastCheckKey(triggerID), bytes)
	c.Send("ZADD", triggersChecksKey, checkData.Score, triggerID)
	c.Send("INCR", selfStateChecksCounterKey)

	if checkData.Score > 0 {
		c.Send("SADD", badStateTriggersKey, triggerID)
	} else {
		c.Send("SREM", badStateTriggersKey, triggerID)
	}

	if _, err = c.Do("EXEC"); err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}

	return nil
}

// RemoveTriggerLastCheck removes trigger last check data
func (connector *DbConnector) RemoveTriggerLastCheck(triggerID string) error {
	c := connector.pool.Get()
	defer c.Close()
	_ = c.Send("MULTI")

	c.Send("DEL", metricLastCheckKey(triggerID))
	c.Send("ZREM", triggersChecksKey, triggerID)
	c.Send("SREM", badStateTriggersKey, triggerID)
	_ = connector.delMaintenanceTrigger(c, triggerID, false)

	_, err := c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}

	return nil
}

// GetTriggerCheckIDs gets checked triggerIDs, sorted from max to min check score and filtered by given tags
// If onlyErrors return only triggerIDs with score > 0
func (connector *DbConnector) GetTriggerCheckIDs(tagNames []string, onlyErrors bool) ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()
	c.Send("MULTI")
	c.Send("ZREVRANGE", triggersChecksKey, 0, -1)
	for _, tagName := range tagNames {
		c.Send("SMEMBERS", tagTriggersKey(tagName))
	}
	if onlyErrors {
		c.Send("SMEMBERS", badStateTriggersKey)
	}
	rawResponse, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, err
	}
	triggerIDs, err := redis.Strings(rawResponse[0], nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve triggers: %s", err.Error())
	}

	triggerIDsByTags := make([]map[string]bool, 0)
	for _, triggersArray := range rawResponse[1:] {
		tagTriggerIDs, err := redis.Strings(triggersArray, nil)
		if err != nil {
			if err == database.ErrNil {
				continue
			}
			return nil, fmt.Errorf("Failed to retrieve tags triggers: %s", err.Error())
		}

		triggerIDsMap := make(map[string]bool)
		for _, triggerID := range tagTriggerIDs {
			triggerIDsMap[triggerID] = true
		}
		triggerIDsByTags = append(triggerIDsByTags, triggerIDsMap)
	}

	total := make([]string, 0)
	for _, triggerID := range triggerIDs {
		valid := true
		for _, triggerIDsByTag := range triggerIDsByTags {
			if _, ok := triggerIDsByTag[triggerID]; !ok {
				valid = false
				break
			}
		}
		if valid {
			total = append(total, triggerID)
		}
	}
	return total, nil
}

var badStateTriggersKey = "moira-bad-state-triggers"
var triggersChecksKey = "moira-triggers-checks"

func metricLastCheckKey(triggerID string) string {
	return fmt.Sprintf("moira-metric-last-check:%s", triggerID)
}
