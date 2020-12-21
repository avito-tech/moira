package reply

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
)

func ScheduledEscalationEvent(rep interface{}, err error) (moira.ScheduledEscalationEvent, error) {
	escalation := moira.ScheduledEscalationEvent{}
	bytes, err := redis.Bytes(rep, err)
	if err != nil {
		if err == redis.ErrNil {
			return escalation, database.ErrNil
		}
		return escalation, fmt.Errorf("Failed to read ScheduledEscalationEvent: %s", err.Error())
	}
	err = json.Unmarshal(bytes, &escalation)
	if err != nil {
		return escalation, fmt.Errorf("Failed to parse ScheduledEscalationEvent json %s: %s", string(bytes), err.Error())
	}
	return escalation, nil
}

func ScheduledEscalationEvents(rep interface{}, err error) ([]*moira.ScheduledEscalationEvent, error) {
	values, err := redis.Values(rep, err)
	if err != nil {
		if err == redis.ErrNil {
			return make([]*moira.ScheduledEscalationEvent, 0), nil
		}
		return nil, fmt.Errorf("Failed to read ScheduledEscalationEvent: %s", err.Error())
	}
	escalations := make([]*moira.ScheduledEscalationEvent, len(values))
	for i, value := range values {
		escalation, err2 := ScheduledEscalationEvent(value, err)
		if err2 != nil && err2 != database.ErrNil {
			return nil, err2
		} else if err2 == database.ErrNil {
			escalations[i] = nil
		} else {
			escalations[i] = &escalation
		}
	}
	return escalations, nil
}
