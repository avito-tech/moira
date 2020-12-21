package reply

import (
	"encoding/json"

	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
)

// Maintenance converts redis DB reply to moira.Maintenance
func Maintenance(reply interface{}, err error) (moira.Maintenance, error) {
	maintenance := moira.NewMaintenance()
	bytes, err := redis.Bytes(reply, err)

	if err != nil {
		if err == redis.ErrNil {
			return maintenance, database.ErrNil
		}
		return nil, errors.Wrap(err, "failed to convert response to bytes")
	}

	err = json.Unmarshal(bytes, &maintenance)
	if err != nil {
		err = errors.Wrap(err, "failed to unmarshal response")
	}

	return maintenance, nil
}
