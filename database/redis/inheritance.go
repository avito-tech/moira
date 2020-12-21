package redis

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

func (connector *DbConnector) AddChildEvents(
	parentTriggerID string, parentMetric string,
	childTriggerID string, childMetrics []string,
) error {
	c := connector.pool.Get()
	defer c.Close()

	encodedParentMetric := encodeTriggerMetric(parentTriggerID, parentMetric)
	c.Send("MULTI")
	argsChild := make([]interface{}, len(childMetrics)+1)
	argsChild[0] = childEventsKey(parentTriggerID, parentMetric)
	for i, childMetric := range childMetrics {
		argsChild[i+1] = encodeTriggerMetric(childTriggerID, childMetric)
		c.Send("SADD", parentEventsKey(childTriggerID, childMetric), encodedParentMetric)
	}
	c.Send("SADD", argsChild...)

	_, err := c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("AddChildEvents failed to EXEC: %s", err.Error())
	}

	return nil
}

func (connector *DbConnector) GetChildEvents(parentTriggerID, parentMetric string) (map[string][]string, error) {
	c := connector.pool.Get()
	defer c.Close()

	key := childEventsKey(parentTriggerID, parentMetric)
	raw, err := redis.Strings(c.Do("SMEMBERS", key))
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to get child events for %s:%s, error: %s",
			parentTriggerID, parentMetric, err.Error(),
		)
	}

	result := make(map[string][]string)
	for _, item := range raw {
		triggerID, metric, err := decodeTriggerMetric(item)
		if err != nil {
			return nil, fmt.Errorf(
				"Failed to decode child event for %s:%s, error: %s",
				parentTriggerID, parentMetric, err.Error(),
			)
		}
		result[triggerID] = append(result[triggerID], metric)
	}
	return result, nil
}

func (connector *DbConnector) GetParentEvents(childTriggerID, childMetric string) (map[string][]string, error) {
	c := connector.pool.Get()
	defer c.Close()

	key := parentEventsKey(childTriggerID, childMetric)
	raw, err := redis.Strings(c.Do("SMEMBERS", key))
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to get child events for %s:%s, error: %s",
			childTriggerID, childMetric, err.Error(),
		)
	}

	result := make(map[string][]string)
	for _, item := range raw {
		triggerID, metric, err := decodeTriggerMetric(item)
		if err != nil {
			return nil, fmt.Errorf(
				"Failed to decode child event for %s:%s, error: %s",
				childTriggerID, childMetric, err.Error(),
			)
		}
		result[triggerID] = append(result[triggerID], metric)
	}
	return result, nil
}

func (connector *DbConnector) DeleteChildEvents(
	parentTriggerID string, parentMetric string,
	childTriggerID string, childMetrics []string,
) error {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	parentEventEncoded := encodeTriggerMetric(parentTriggerID, parentMetric)
	for _, childMetric := range childMetrics {
		key := parentEventsKey(childTriggerID, childMetric)
		c.Send("SREM", key, parentEventEncoded)

		childEventEncoded := encodeTriggerMetric(childTriggerID, childMetric)
		c.Send("SREM", childEventsKey(parentTriggerID, parentMetric), childEventEncoded)
	}

	_, err := c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("DeleteChildEvents failed to EXEC: %s", err.Error())
	}

	return nil
}

func (connector *DbConnector) UpdateInheritanceDataVersion() error {
	c := connector.pool.Get()
	defer c.Close()

	datePart := time.Now().Format("2006-01-02:15:04:05")
	randomPart := rand.Uint32()
	newVersion := fmt.Sprintf("%s:%x", datePart, randomPart)

	_, err := c.Do("SET", inheritanceDataVersionKey, newVersion)
	if err != nil {
		return fmt.Errorf("Failed to update inheritance data version: %s", err.Error())
	}
	return nil
}

// encodeTriggerMetric stores a trigger ID and a metric name as a single string
func encodeTriggerMetric(triggerID, metric string) string {
	return fmt.Sprintf("%s:%s", triggerID, metric)
}

// decodeTriggerMetric parses a string generated by `encodeTriggerMetric`
func decodeTriggerMetric(s string) (triggerID, metric string, err error) {
	split := strings.SplitN(s, ":", 2)
	if len(split) < 2 {
		return "", "", fmt.Errorf("decodeTriggerMetric failed to parse %s", s)
	}
	return split[0], split[1], nil
}

func childEventsKey(triggerID, metric string) string {
	// childEventsKey(ID, m) stores all children of event in trigger ID, metric m
	return fmt.Sprintf("moira-inheritance-child-events:%s:%s", triggerID, metric)
}

func parentEventsKey(triggerID, metric string) string {
	// parentEventsKey(ID, m) stores all parents of event in trigger ID, metric m
	return fmt.Sprintf("moira-inheritance-parent-events:%s:%s", triggerID, metric)
}

const inheritanceDataVersionKey = "moira-inheritance-data-version"