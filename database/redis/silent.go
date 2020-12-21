package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"

	"go.avito.ru/DO/moira"
)

const (
	silentPatternKey         = "moira-silent-patterns"
	silentPatternLockKey     = "moira-silent-patterns-lock"
	silentPatternLockTimeout = 6 * time.Second
	silentPatternLockTtlSec  = 30
)

func (connector *DbConnector) GetSilentPatternsAll() ([]*moira.SilentPatternData, error) {
	c := connector.pool.Get()
	defer c.Close()

	rep, err := redis.Values(c.Do("HGETALL", silentPatternKey))
	result := make([]*moira.SilentPatternData, 0, len(rep)/2)
	if err != nil {
		return result, fmt.Errorf("Failed to HGETALL: %s", err.Error())
	}

	for i, r := range rep {
		if i%2 == 1 { // even elements is a key
			el := &moira.SilentPatternData{}
			b, err := redis.Bytes(r, nil)
			if err != nil {
				return result, err
			}
			if err = json.Unmarshal(b, el); err != nil {
				return result, err
			}
			result = append(result, el)
		}
	}
	return result, nil
}

// GetSilentPatternsTyped extract only silent patterns of expected type
func (connector *DbConnector) GetSilentPatternsTyped(pt moira.SilentPatternType) ([]*moira.SilentPatternData, error) {
	patternsAll, err := connector.GetSilentPatternsAll()
	result := make([]*moira.SilentPatternData, 0, len(patternsAll))

	for _, patternData := range patternsAll {
		if patternData.Type == pt {
			result = append(result, patternData)
		}
	}

	return result, err
}

func (connector *DbConnector) SaveSilentPatterns(pt moira.SilentPatternType, spl ...*moira.SilentPatternData) error {
	if len(spl) == 0 {
		return nil
	}

	c := connector.pool.Get()
	defer c.Close()

	maintenance, err := connector.getOrCreateMaintenanceSilent(c, pt)
	if err != nil {
		return errors.Wrapf(err, "Failed to get maintenance (type %d)", pt)
	}

	_ = c.Send("MULTI")
	for _, sp := range spl {
		if sp.ID == "" {
			sp.ID = moira.NewStrID()
		}

		payload, err := json.Marshal(sp)
		if err != nil {
			return errors.Wrap(err, "Failed to marshal")
		}

		if err = c.Send("HSET", silentPatternKey, sp.ID, payload); err != nil {
			return errors.Wrap(err, "Failed to HSET")
		}
		maintenance.Add(sp.Pattern, sp.Until)
	}

	if err = connector.setMaintenanceSilent(c, pt, maintenance, false); err != nil {
		return errors.Wrapf(err, "Failed to set maintenance (type %d)", pt)
	}

	_, err = c.Do("EXEC")
	if err != nil {
		err = errors.Wrap(err, "Failed to EXEC")
	}

	return err
}

func (connector *DbConnector) RemoveSilentPatterns(pt moira.SilentPatternType, spl ...*moira.SilentPatternData) error {
	if len(spl) == 0 {
		return nil
	}

	c := connector.pool.Get()
	defer c.Close()

	maintenance, err := connector.getOrCreateMaintenanceSilent(c, pt)
	if err != nil {
		return errors.Wrapf(err, "Failed to get maintenance (type %d)", pt)
	}

	_ = c.Send("MULTI")
	for _, sp := range spl {
		if err = c.Send("HDEL", silentPatternKey, sp.ID); err != nil {
			return errors.Wrap(err, "Failed to HDEL")
		}
		maintenance.Del(sp.Pattern)
	}

	if err = connector.setMaintenanceSilent(c, pt, maintenance, false); err != nil {
		return errors.Wrapf(err, "Failed to set maintenance (type %d)", pt)
	}

	_, err = c.Do("EXEC")
	if err != nil {
		err = errors.Wrap(err, "Failed to EXEC")
	}

	return err
}

func (connector *DbConnector) LockSilentPatterns(pt moira.SilentPatternType) error {
	return connector.AcquireLock(
		silentPatternLockKeyTyped(pt),
		silentPatternLockTtlSec,
		silentPatternLockTimeout,
	)
}

func (connector *DbConnector) UnlockSilentPatterns(pt moira.SilentPatternType) error {
	return connector.DeleteLock(silentPatternLockKeyTyped(pt))
}

func silentPatternLockKeyTyped(pt moira.SilentPatternType) string {
	return fmt.Sprintf("%s:%d", silentPatternLockKey, pt)
}
