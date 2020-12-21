package reply

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
)

func GlobalSettings(reply interface{}, err error) (moira.GlobalSettings, error) {
	globalSettings := moira.GlobalSettings{}
	if bytes, err := redis.Bytes(reply, err); err != nil {
		if err == redis.ErrNil {
			return globalSettings, database.ErrNil
		} else {
			return globalSettings, fmt.Errorf("Failed to read global settings: %s", err.Error())
		}
	} else if err := json.Unmarshal(bytes, &globalSettings); err != nil {
		return globalSettings, fmt.Errorf("Failed to parse global settings: %s", err.Error())
	} else {
		return globalSettings, nil
	}
}
