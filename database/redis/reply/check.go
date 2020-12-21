package reply

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
)

// Check converts redis DB reply to *moira.CheckData
func Check(rep interface{}, err error) (*moira.CheckData, error) {
	checkData := &moira.CheckData{}
	bytes, err := redis.Bytes(rep, err)

	if err != nil {
		if err == redis.ErrNil {
			return nil, database.ErrNil
		}
		return nil, fmt.Errorf("Failed to read lastCheck: %s", err.Error())
	}

	err = json.Unmarshal(bytes, checkData)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse lastCheck json %s: %s", string(bytes), err.Error())
	}

	return checkData, nil
}
