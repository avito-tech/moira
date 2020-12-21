package redis

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database/redis/reply"
)

const (
	keyScheduledEscalations           = "moira-notifier-scheduled-escalations"
	prefixPendingEscalation           = "moira-trigger-pending-escalations"
	prefixPendingEscalationResolution = "moira-trigger-pending-escalations-res"
	prefixProcessedEscalation         = "moira-trigger-processed-escalations"
)

func (connector *DbConnector) AddEscalations(
	ts int64, event moira.NotificationEvent, trigger moira.TriggerData, escalations []moira.EscalationData,
) error {
	c := connector.pool.Get()
	defer c.Close()

	var (
		err    error
		offset int64
	)

	// if event is about resolution then it's necessary to remove escalations
	// and vice versa: if event is about an error then it's necessary to remove resolutions
	isResolution := event.State == moira.OK
	_ = connector.ackTriggerEscalations(trigger.ID, event.Metric, !isResolution)

	c.Send("MULTI")
	c.Send("SET", triggerPendingEscalationsKey(trigger.ID, event.Metric, isResolution), "")

	// resolution-type escalations must be filtered by those ones which were sent
	if isResolution {
		if escalations, err = connector.filterEscalationsByRegistered(escalations, event.Metric, trigger.ID); err != nil {
			return err
		}
	}

	lastIndex := len(escalations) - 1
	for i, e := range escalations {
		scheduledEvent := &moira.ScheduledEscalationEvent{
			Escalation:   e,
			Event:        event,
			Trigger:      trigger,
			IsFinal:      i == lastIndex,
			IsResolution: isResolution,
		}
		bytes, err := json.Marshal(scheduledEvent)
		if err != nil {
			return err
		}

		if !isResolution {
			// if event is not about problem being resolved then escalation is needed according to the schedule
			offset = e.OffsetInMinutes * 60
		} else {
			// otherwise (if problem is resolved) escalation can be sent right now
			offset = 0
		}
		c.Send("ZADD", keyScheduledEscalations, ts+offset, bytes)
	}

	if _, err := c.Do("EXEC"); err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	return nil
}

func (connector *DbConnector) TriggerHasPendingEscalations(triggerID string, withResolutions bool) (bool, error) {
	c := connector.pool.Get()
	defer c.Close()

	for _, isResolution := range []bool{false, true} {
		if isResolution && !withResolutions {
			continue
		} else {
			value, err := redis.Strings(c.Do("KEYS", triggerPendingEscalationsKey(triggerID, "*", isResolution)))
			if err != nil && err != redis.ErrNil {
				return false, err
			}
			if len(value) > 0 {
				return true, nil
			}
		}
	}

	return false, nil
}

func (connector *DbConnector) MetricHasPendingEscalations(triggerID, metric string, withResolutions bool) (bool, error) {
	c := connector.pool.Get()
	defer c.Close()

	for _, isResolution := range []bool{false, true} {
		if isResolution && !withResolutions {
			continue
		} else if value, err := redis.Bool(c.Do("EXISTS", triggerPendingEscalationsKey(triggerID, metric, isResolution))); err != nil || value {
			return value, err
		}
	}

	return false, nil
}

func (connector *DbConnector) AckEscalations(triggerID, metric string, withResolutions bool) error {
	for _, isResolution := range []bool{false, true} {
		if isResolution && !withResolutions {
			continue
		} else if err := connector.ackTriggerEscalations(triggerID, metric, isResolution); err != nil {
			return err
		}
	}

	return nil
}

func (connector *DbConnector) AckEscalationsBatch(triggerID string, metrics []string, withResolutions bool) error {
	for _, isResolution := range []bool{false, true} {
		if isResolution && !withResolutions {
			continue
		} else if err := connector.ackTriggerEscalationsBatch(triggerID, metrics, isResolution); err != nil {
			return err
		}
	}

	return nil
}

func (connector *DbConnector) FetchScheduledEscalationEvents(to int64) ([]*moira.ScheduledEscalationEvent, error) {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("ZRANGEBYSCORE", keyScheduledEscalations, "-inf", to)
	c.Send("ZREMRANGEBYSCORE", keyScheduledEscalations, "-inf", to)
	response, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %s", err)
	}
	if len(response) == 0 {
		return make([]*moira.ScheduledEscalationEvent, 0), nil
	}
	return reply.ScheduledEscalationEvents(response[0], nil)
}

// RegisterProcessedEscalation stores data of escalation (but not resolution) event that was processed (sent)
func (connector *DbConnector) RegisterProcessedEscalationID(escalationID, triggerID, metric string) error {
	c := connector.pool.Get()
	defer c.Close()

	_, err := c.Do("LPUSH", triggerProcessedEscalationsKey(triggerID, metric), escalationID)
	return err
}

func (connector *DbConnector) ackTriggerEscalations(triggerID, metric string, isResolution bool) error {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("DEL", triggerPendingEscalationsKey(triggerID, metric, isResolution))
	if isResolution {
		// the metric is now OK, so we can delete all unacked messages
		c.Send("DEL", unacknowledgedMessagesKey(triggerID, metric))
	}
	_, err := c.Do("EXEC")
	return err
}

func (connector *DbConnector) ackTriggerEscalationsBatch(triggerID string, metrics []string, isResolution bool) error {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")

	// cutting these into chunks so that Redis doesn't choke
	for len(metrics) > 0 {
		const chunkSize = 25
		var chunk, rest []string = metrics, nil
		if len(chunk) > chunkSize {
			chunk, rest = chunk[:chunkSize], chunk[chunkSize:]
		}

		peKeys := make([]interface{}, len(chunk))
		for i, metric := range chunk {
			peKeys[i] = triggerPendingEscalationsKey(triggerID, metric, isResolution)
		}
		c.Send("DEL", peKeys...)

		if isResolution {
			// the metric is now OK, so we can delete all unacked messages
			umKeys := make([]interface{}, len(chunk))
			for i, metric := range chunk {
				umKeys[i] = unacknowledgedMessagesKey(triggerID, metric)
			}
			c.Send("DEL", umKeys...)
		}

		metrics = rest
	}

	_, err := c.Do("EXEC")
	return err
}

func (connector *DbConnector) fetchRegisteredEscalationIDs(triggerID, metric string) ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("LRANGE", triggerProcessedEscalationsKey(triggerID, metric), 0, -1)
	c.Send("DEL", triggerProcessedEscalationsKey(triggerID, metric))

	if rawResponse, err := redis.Values(c.Do("EXEC")); err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %s", err.Error())
	} else {
		return redis.Strings(rawResponse[0], nil)
	}
}

func (connector *DbConnector) filterEscalationsByRegistered(escalations []moira.EscalationData, triggerID, metric string) ([]moira.EscalationData, error) {
	if registeredEscalationIDs, err := connector.fetchRegisteredEscalationIDs(triggerID, metric); err != nil {
		return nil, err
	} else {
		filteredEscalations := make([]moira.EscalationData, 0, len(escalations))
		registeredEscalationSet := make(map[string]bool)

		// create map of registered escalation ids
		for _, id := range registeredEscalationIDs {
			registeredEscalationSet[id] = true
		}

		// keep only registered escalations
		for _, escalation := range escalations {
			if _, ok := registeredEscalationSet[escalation.ID]; ok {
				filteredEscalations = append(filteredEscalations, escalation)
			}
		}

		return filteredEscalations, nil
	}
}

func triggerPendingEscalationsKey(triggerID, metric string, isResolution bool) string {
	var prefix string
	if !isResolution {
		prefix = prefixPendingEscalation
	} else {
		prefix = prefixPendingEscalationResolution
	}

	return fmt.Sprintf("%s:%s:%s", prefix, triggerID, metric)
}

func triggerProcessedEscalationsKey(triggerID, metric string) string {
	return fmt.Sprintf("%s:%s:%s", prefixProcessedEscalation, triggerID, metric)
}

func (connector *DbConnector) AddUnacknowledgedMessage(
	triggerID string, metric string, link moira.MessageLink,
) error {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	key := unacknowledgedMessagesKey(triggerID, metric)
	c.Send("SADD", key, link.StorageKey())
	_, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err)
	}
	return nil
}

func (connector *DbConnector) GetUnacknowledgedMessages(triggerID, metric string) ([]moira.MessageLink, error) {
	c := connector.pool.Get()
	defer c.Close()

	key := unacknowledgedMessagesKey(triggerID, metric)
	response, err := redis.Strings(c.Do("SMEMBERS", key))
	if err != nil {
		return nil, fmt.Errorf("could not get acknowledged messages: %s", err.Error())
	}
	parsedResponse := make([]moira.MessageLink, len(response))
	for i, item := range response {
		link, err := parseMessageLink(item)
		if err != nil {
			return nil, fmt.Errorf("could not get acknowledged messages: %s", err.Error())
		}
		parsedResponse[i] = link
	}
	return parsedResponse, nil
}

func (connector *DbConnector) AckUnacknowledgedMessages(triggerID, metric string) error {
	c := connector.pool.Get()
	defer c.Close()

	_, err := c.Do("DEL", unacknowledgedMessagesKey(triggerID, metric))
	return err
}

func parseMessageLink(s string) (moira.MessageLink, error) {
	split := strings.SplitN(s, ":", 2)
	if len(split) < 2 {
		return nil, fmt.Errorf("%s is not a valid MessageLink", s)
	}

	switch split[0] {
	case "slack":
		link := new(moira.SlackThreadLink)
		if err := link.FromString(split[1]); err != nil {
			return nil, err
		} else {
			return link, nil
		}
	default:
		return nil, fmt.Errorf("%s is not a known MessageLink type", split[0])
	}
}

func unacknowledgedMessagesKey(triggerID, metric string) string {
	return fmt.Sprintf("moira-unacknowledged-messages:%s:%s", triggerID, metric)
}
