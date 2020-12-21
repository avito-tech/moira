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

const (
	globalSettingsKey     = "moira-global-settings"
	globalSettingsLockKey = "moira-global-settings-lock"
)

func (connector *DbConnector) GetGlobalSettings() (moira.GlobalSettings, error) {
	c := connector.pool.Get()
	defer c.Close()

	result, err := reply.GlobalSettings(c.Do("GET", globalSettingsKey))
	if err == database.ErrNil {
		err = nil
	}

	return result, err
}

func (connector *DbConnector) SetGlobalSettings(newSettings moira.GlobalSettings) error {
	c := connector.pool.Get()
	defer c.Close()

	if err := connector.setGlobalSettingsLock(); err != nil {
		return err
	}
	defer connector.releaseGlobalSettingsLock()

	if bytes, err := json.Marshal(&newSettings); err != nil {
		return err
	} else if _, err := c.Do("SET", globalSettingsKey, bytes); err != nil {
		return fmt.Errorf("Failed to SET: %s", err.Error())
	} else {
		return nil
	}
}

func (connector *DbConnector) setGlobalSettingsLock() error {
	c := connector.pool.Get()
	defer c.Close()

	if _, err := redis.String(c.Do("SET", globalSettingsLockKey, time.Now().Unix(), "EX", 5, "NX")); err != nil {
		if err == redis.ErrNil {
			return fmt.Errorf("Failed to set global settings lock: lock is busy")
		} else {
			return fmt.Errorf("Failed to set global settings lock: %s", err.Error())
		}
	} else {
		return nil
	}
}

func (connector *DbConnector) releaseGlobalSettingsLock() error {
	c := connector.pool.Get()
	defer c.Close()

	if _, err := c.Do("DEL", globalSettingsLockKey); err != nil {
		return fmt.Errorf("Failed to delete global settings lock: %s", err.Error())
	} else {
		return nil
	}
}
