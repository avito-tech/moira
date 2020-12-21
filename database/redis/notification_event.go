package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/database/redis/reply"
)

var eventsTTL int64 = 3600 * 24 * 30

// GetNotificationEvents gets NotificationEvents by given triggerID and interval
func (connector *DbConnector) GetNotificationEvents(triggerID string, start int64, size int64) ([]*moira.NotificationEvent, error) {
	c := connector.pool.Get()
	defer c.Close()

	eventsData, err := reply.Events(c.Do("ZREVRANGE", triggerEventsKeyCommon(triggerID), start, start+size))
	if err != nil {
		if err == redis.ErrNil {
			return make([]*moira.NotificationEvent, 0), nil
		}
		return nil, fmt.Errorf("Failed to get range for trigger events, triggerID: %s, error: %s", triggerID, err.Error())
	}

	return eventsData, nil
}

// GetAllNotificationEvents gets all NotificationEvents by given interval
func (connector *DbConnector) GetAllNotificationEvents(start, end int64) ([]*moira.NotificationEvent, error) {
	c := connector.pool.Get()
	defer c.Close()

	eventsData, err := reply.Events(c.Do("ZRANGEBYSCORE", allEventsLogKey, start, end))
	if err != nil {
		if err == redis.ErrNil {
			return make([]*moira.NotificationEvent, 0), nil
		}
		return nil, fmt.Errorf("Failed to get all events, error: %s", err.Error())
	}

	return eventsData, nil
}

// PushNotificationEvent adds new NotificationEvent to events list and to given triggerID events list and deletes events that are older than 30 days
// If ui=true, then add to ui events list
func (connector *DbConnector) PushNotificationEvent(event *moira.NotificationEvent) error {
	if event.ID == "" {
		event.ID = event.IdempotencyKey()
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	c := connector.pool.Get()
	defer c.Close()

	var eventKey string
	if event.HasSaturations {
		eventKey = eventsWithSaturationsListKey
	} else {
		eventKey = eventsListKey
	}

	c.Send("MULTI")
	c.Send("LPUSH", eventKey, eventBytes)

	if event.TriggerID != "" {
		removeEventsUntil := time.Now().Unix() - eventsTTL
		triggerEventKeys := []string{triggerEventsKeyCommon(event.TriggerID), allEventsLogKey}

		for _, key := range triggerEventKeys {
			c.Send("ZADD", key, event.Timestamp, eventBytes)
			c.Send("ZREMRANGEBYSCORE", key, "-inf", removeEventsUntil) // housekeeping
		}
	}

	_, err = c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}

	return nil
}

// GetNotificationEventCount returns planned notifications count from given timestamp
func (connector *DbConnector) GetNotificationEventCount(triggerID string, from int64) int64 {
	c := connector.pool.Get()
	defer c.Close()

	count, _ := redis.Int64(c.Do("ZCOUNT", triggerEventsKeyCommon(triggerID), from, "+inf"))
	return count
}

// FetchNotificationEvent waiting for event in events list
func (connector *DbConnector) FetchNotificationEvent(withSaturations bool) (moira.NotificationEvent, error) {
	c := connector.pool.Get()
	defer c.Close()

	var event moira.NotificationEvent

	var eventKey string
	if withSaturations {
		eventKey = eventsWithSaturationsListKey
	} else {
		eventKey = eventsListKey
	}

	rawRes, err := c.Do("BRPOP", eventKey, 1)
	if err != nil {
		return event, fmt.Errorf("Failed to fetch event: %s", err.Error())
	}
	if rawRes == nil {
		return event, database.ErrNil
	}
	res, _ := redis.Values(rawRes, nil)

	var (
		eventBytes []byte
		key        []byte
	)
	if _, err = redis.Scan(res, &key, &eventBytes); err != nil {
		return event, fmt.Errorf("Failed to parse event: %s", err.Error())
	}
	if err := json.Unmarshal(eventBytes, &event); err != nil {
		return event, fmt.Errorf("Failed to parse event json %s: %s", eventBytes, err.Error())
	}
	return event, nil
}

func (connector *DbConnector) FetchDelayedNotificationEvents(to int64, withSaturations bool) ([]moira.NotificationEvent, error) {
	c := connector.pool.Get()
	defer c.Close()

	var eventKey string
	if withSaturations {
		eventKey = delayedEventsWithSaturationsListKey
	} else {
		eventKey = delayedEventsListKey
	}

	c.Send("MULTI")
	c.Send("ZRANGEBYSCORE", eventKey, "-inf", to)
	c.Send("ZREMRANGEBYSCORE", eventKey, "-inf", to)
	response, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %s", err)
	}

	res, _ := redis.ByteSlices(response[0], nil)
	if len(res) == 0 {
		return make([]moira.NotificationEvent, 0), nil
	}

	events := make([]moira.NotificationEvent, len(res))
	for i, eventBytes := range res {
		if err := json.Unmarshal(eventBytes, &(events[i])); err != nil {
			return nil, fmt.Errorf("Failed to parse notificationEvent json %s: %s", eventBytes, err.Error())
		}
	}
	return events, nil
}

func (connector *DbConnector) AddDelayedNotificationEvent(event moira.NotificationEvent, timestamp int64) error {
	c := connector.pool.Get()
	defer c.Close()

	var eventKey string
	if event.HasSaturations {
		eventKey = delayedEventsWithSaturationsListKey
	} else {
		eventKey = delayedEventsListKey
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("could not serialize notificationEvent to json: %s", err.Error())
	}
	_, err = c.Do("ZADD", eventKey, timestamp, bytes)
	if err != nil {
		return fmt.Errorf("failed to save delayed notificationEvent: %s", err.Error())
	}
	return nil
}

const (
	eventsListKey                = "moira-trigger-events"
	eventsWithSaturationsListKey = eventsListKey + ":with-saturations"

	delayedEventsListKey                = "moira-trigger-delayed-events"
	delayedEventsWithSaturationsListKey = delayedEventsListKey + ":with-saturations"

	allEventsLogKey = "moira-all-events"
)

func triggerEventsKeyCommon(triggerID string) string {
	return fmt.Sprintf("moira-trigger-events:%s", triggerID)
}
