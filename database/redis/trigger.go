package redis

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/database/redis/reply"
)

// GetTriggerIDs finds out whether trigger exists
func (connector *DbConnector) CheckTriggerExists(triggerID string) (bool, error) {
	c := connector.pool.Get()
	defer c.Close()

	return redis.Bool(c.Do("EXISTS", triggerKey(triggerID)))
}

// GetTriggerIDs gets all moira triggerIDs, if no value, return database.ErrNil error
func (connector *DbConnector) GetTriggerIDs(onlyPull bool) ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()
	key := triggersListKey
	if onlyPull {
		key = pullTriggersListKey
	}
	triggerIds, err := redis.Strings(c.Do("SMEMBERS", key))
	if err != nil {
		return nil, fmt.Errorf("Failed to get triggers-list: %s", err.Error())
	}
	return triggerIds, nil
}

// GetTrigger gets trigger and trigger tags by given ID and return it in merged object
func (connector *DbConnector) GetTrigger(triggerID string) (*moira.Trigger, error) {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("GET", triggerKey(triggerID))
	c.Send("SMEMBERS", triggerTagsKey(triggerID))
	rawResponse, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	return connector.getTriggerWithTags(rawResponse[0], rawResponse[1], triggerID)
}

// GetTriggers returns triggers data by given ids, len of triggerIDs is equal to len of returned values array.
// If there is no object by current ID, then nil is returned
func (connector *DbConnector) GetTriggers(triggerIDs []string) ([]*moira.Trigger, error) {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	for _, triggerID := range triggerIDs {
		c.Send("GET", triggerKey(triggerID))
		c.Send("SMEMBERS", triggerTagsKey(triggerID))
	}
	rawResponse, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %s", err.Error())
	}

	triggers := make([]*moira.Trigger, len(triggerIDs))
	for i := 0; i < len(rawResponse); i += 2 {
		triggerID := triggerIDs[i/2]
		trigger, err := connector.getTriggerWithTags(rawResponse[i], rawResponse[i+1], triggerID)
		if err != nil {
			if err == database.ErrNil {
				continue
			}
			return nil, err
		}
		triggers[i/2] = trigger
	}
	return triggers, nil
}

// GetPatternTriggerIDs gets trigger list by given pattern
func (connector *DbConnector) GetPatternTriggerIDs(pattern string) ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()

	triggerIds, err := redis.Strings(c.Do("SMEMBERS", patternTriggersKey(pattern)))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve pattern triggers for pattern: %s, error: %s", pattern, err.Error())
	}
	return triggerIds, nil
}

// RemovePatternTriggerIDs removes all triggerIDs list accepted to given pattern
func (connector *DbConnector) RemovePatternTriggerIDs(pattern string) error {
	c := connector.pool.Get()
	defer c.Close()
	_, err := c.Do("DEL", patternTriggersKey(pattern))
	if err != nil {
		return fmt.Errorf("Failed delete pattern-triggers: %s, error: %s", pattern, err)
	}
	return nil
}

// SaveTrigger sets trigger data by given trigger and triggerID
// If trigger already exists, then merge old and new trigger patterns and tags list
// and cleanup not used tags and patterns from lists
// If given trigger contains new tags then create it
func (connector *DbConnector) SaveTrigger(triggerID string, trigger *moira.Trigger) error {
	existing, errGetTrigger := connector.GetTrigger(triggerID)
	if errGetTrigger != nil && errGetTrigger != database.ErrNil {
		return errGetTrigger
	}
	bytes, err := reply.GetTriggerBytes(triggerID, trigger)
	if err != nil {
		return err
	}
	c := connector.pool.Get()
	defer c.Close()
	c.Send("MULTI")
	cleanupPatterns := make([]string, 0)

	// existing trigger found
	if errGetTrigger != database.ErrNil {
		patterns := existing.Patterns
		if trigger.IsPullType {
			c.Do("SREM", triggersListKey, triggerID)
		} else {
			patterns = leftJoin(existing.Patterns, trigger.Patterns)
			c.Do("SREM", pullTriggersListKey, triggerID)
		}

		for _, pattern := range patterns {
			c.Send("SREM", patternTriggersKey(pattern), triggerID)
			cleanupPatterns = append(cleanupPatterns, pattern)
		}
		for _, tag := range leftJoin(existing.Tags, trigger.Tags) {
			c.Send("SREM", triggerTagsKey(triggerID), tag)
			c.Send("SREM", tagTriggersKey(tag), triggerID)
		}
	}
	c.Do("SET", triggerKey(triggerID), bytes)

	c.Do("SADD", triggersListKey, triggerID)
	if trigger.IsPullType {
		c.Do("SADD", pullTriggersListKey, triggerID)
	} else {
		for _, pattern := range trigger.Patterns {
			c.Do("SADD", patternsListKey, pattern)
			c.Do("SADD", patternTriggersKey(pattern), triggerID)
		}
	}

	for _, tag := range trigger.Tags {
		c.Send("SADD", triggerTagsKey(triggerID), tag)
		c.Send("SADD", tagTriggersKey(tag), triggerID)
		c.Send("SADD", tagsKey, tag)
	}
	_, err = c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	for _, pattern := range cleanupPatterns {
		triggerIDs, err := connector.GetPatternTriggerIDs(pattern)
		if err != nil {
			return err
		}
		if len(triggerIDs) == 0 {
			connector.RemovePatternTriggerIDs(pattern)
			connector.RemovePattern(pattern)
			connector.RemovePatternsMetrics([]string{pattern})
		}
	}
	return nil
}

// RemoveTrigger deletes trigger data by given triggerID, delete trigger tag list,
// Deletes triggerID from containing tags triggers list and from containing patterns triggers list
// If containing patterns doesn't used in another triggers, then delete this patterns with metrics data
func (connector *DbConnector) RemoveTrigger(triggerID string) error {
	trigger, err := connector.GetTrigger(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return nil
		}
		return err
	}

	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("DEL", triggerKey(triggerID))
	c.Send("DEL", triggerTagsKey(triggerID))
	c.Send("SREM", triggersListKey, triggerID)
	c.Send("SREM", pullTriggersListKey, triggerID)
	for _, tag := range trigger.Tags {
		c.Send("SREM", tagTriggersKey(tag), triggerID)
	}
	for _, pattern := range trigger.Patterns {
		c.Send("SREM", patternTriggersKey(pattern), triggerID)
	}
	_, err = c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}

	for _, pattern := range trigger.Patterns {
		count, err := redis.Int64(c.Do("SCARD", patternTriggersKey(pattern)))
		if err != nil {
			return fmt.Errorf("Failed to SCARD pattern triggers: %s", err.Error())
		}
		if count == 0 {
			if err := connector.RemovePatternWithMetrics(pattern); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetTriggerChecks gets triggers data with tags, lastCheck data and throttling by given triggersIDs
// Len of triggerIDs is equal to len of returned values array.
// If there is no object by current ID, then nil is returned
func (connector *DbConnector) GetTriggerChecks(triggerIDs []string) ([]*moira.TriggerCheck, error) {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	for _, triggerID := range triggerIDs {
		c.Send("GET", triggerKey(triggerID))
		c.Send("SMEMBERS", triggerTagsKey(triggerID))
		c.Send("GET", metricLastCheckKey(triggerID))
		c.Send("GET", notifierNextKey(triggerID))
	}
	rawResponse, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %s", err)
	}
	var slices [][]interface{}
	for i := 0; i < len(rawResponse); i += 4 {
		arr := make([]interface{}, 0, 5)
		arr = append(arr, triggerIDs[i/4])
		arr = append(arr, rawResponse[i:i+4]...)
		slices = append(slices, arr)
	}
	triggerChecks := make([]*moira.TriggerCheck, len(slices))
	for i, slice := range slices {
		triggerID := slice[0].(string)
		trigger, err := connector.getTriggerWithTags(slice[1], slice[2], triggerID)
		if err != nil {
			if err == database.ErrNil {
				continue
			}
			return nil, err
		}
		lastCheck, err := reply.Check(slice[3], nil)
		if err != nil && err != database.ErrNil {
			return nil, err
		}
		throttling, _ := redis.Int64(slice[4], nil)
		if time.Now().Unix() >= throttling {
			throttling = 0
		}
		triggerChecks[i] = &moira.TriggerCheck{
			Trigger:    *trigger,
			LastCheck:  lastCheck,
			Throttling: throttling,
		}
	}
	return triggerChecks, nil
}

func (connector *DbConnector) AddTriggerForcedNotification(triggerID string, metrics []string, time int64) error {
	c := connector.pool.Get()
	defer c.Close()

	args := make([]interface{}, len(metrics)*2+3)
	args[0] = triggerForcedNotificationsKey(triggerID)
	args[1] = moira.WildcardMetric
	args[2] = time
	for i, metric := range metrics {
		args[2*i+3] = metric
		args[2*i+4] = time
	}

	_, err := c.Do("HSET", args...)
	if err != nil {
		return fmt.Errorf(
			"Failed to add forced notifications to Redis, args: %+v: %s",
			args,
			err.Error(),
		)
	}
	return nil
}

func (connector *DbConnector) GetTriggerForcedNotifications(triggerID string) (map[string]bool, error) {
	c := connector.pool.Get()
	defer c.Close()

	result := make(map[string]bool)
	key := triggerForcedNotificationsKey(triggerID)
	forcedNotificationTimes, err := redis.Int64Map(c.Do("HGETALL", key))
	switch {
	case err == redis.ErrNil:
		return result, nil
	case err != nil:
		return nil, err
	}

	now := time.Now().Unix()
	for metric, forcedNotificationTime := range forcedNotificationTimes {
		if forcedNotificationTime <= now {
			result[metric] = true
		}
	}
	return result, nil
}

func (connector *DbConnector) DeleteTriggerForcedNotification(triggerID string, metric string) error {
	return connector.DeleteTriggerForcedNotifications(triggerID, []string{metric})
}

func (connector *DbConnector) DeleteTriggerForcedNotifications(triggerID string, metrics []string) error {
	c := connector.pool.Get()
	defer c.Close()

	key := triggerForcedNotificationsKey(triggerID)
	args := make([]interface{}, len(metrics)+1)
	args[0] = key
	for i, metric := range metrics {
		args[i+1] = metric
	}
	_, err := c.Do("HDEL", args...)
	if err != nil {
		return fmt.Errorf("Failed to delete forced notification from Redis: %s", err.Error())
	}
	return nil
}

func (connector *DbConnector) getTriggerWithTags(triggerRaw interface{}, tagsRaw interface{}, triggerID string) (*moira.Trigger, error) {
	trigger, err := reply.Trigger(triggerRaw, nil)
	if err != nil {
		return trigger, err
	}
	triggerTags, err := redis.Strings(tagsRaw, nil)
	if err != nil {
		connector.logger.ErrorF("Error getting trigger tags, id: %s, error: %s", triggerID, err.Error())
	}
	if len(triggerTags) > 0 {
		trigger.Tags = triggerTags
	}
	trigger.ID = triggerID
	return trigger, nil
}

func leftJoin(left, right []string) []string {
	rightValues := make(map[string]bool)
	for _, value := range right {
		rightValues[value] = true
	}
	arr := make([]string, 0)
	for _, leftValue := range left {
		if _, ok := rightValues[leftValue]; !ok {
			arr = append(arr, leftValue)
		}
	}
	return arr
}

var triggersListKey = "moira-triggers-list"
var pullTriggersListKey = "moira-pull-triggers-list"

func triggerCheckLogStatsKey(triggerID string) string {
	return fmt.Sprintf("moira-trigger-check-log-stats:%s", triggerID)
}

func triggerForcedNotificationsKey(triggerID string) string {
	return fmt.Sprintf("moira-triggers-forced-notifications:%s", triggerID)
}

func triggerKey(triggerID string) string {
	return fmt.Sprintf("moira-trigger:%s", triggerID)
}

func triggerTagsKey(triggerID string) string {
	return fmt.Sprintf("moira-trigger-tags:%s", triggerID)
}

func patternTriggersKey(pattern string) string {
	return fmt.Sprintf("moira-pattern-triggers:%s", pattern)
}
