package redis

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
)

const (
	lockAcquireGranularity = 200 * time.Millisecond
)

// AcquireLock sets lock by given key repeatedly until success or expiration of the given timeout
func (connector *DbConnector) AcquireLock(lockKey string, ttlSec int, timeout time.Duration) error {
	c := connector.pool.Get()
	defer c.Close()

	start := time.Now()
	for {
		if acquired, err := connector.setLock(c, lockKey, ttlSec); acquired || err != nil {
			return err
		}

		if time.Since(start) > timeout {
			return fmt.Errorf(
				"Could not acquire lock with key \"%s\" during %s",
				lockKey,
				timeout.String(),
			)
		}

		time.Sleep(lockAcquireGranularity)
	}
}

// AcquireTriggerCheckLock is the special case of AcquireLock for the give trigger id
func (connector *DbConnector) AcquireTriggerCheckLock(triggerID string) error {
	return connector.AcquireLock(
		triggerLockKey(triggerID),
		30,
		10*lockAcquireGranularity,
	)
}

// AcquireTriggerMaintenanceLock is the special case of AcquireLock for the give trigger id
func (connector *DbConnector) AcquireTriggerMaintenanceLock(triggerID string) error {
	return connector.AcquireLock(
		triggerMaintenanceLockKey(triggerID),
		30,
		10*lockAcquireGranularity,
	)
}

// SetLock create to database lock object with given TTL and return true if object successfully created, or false if object already exists
func (connector *DbConnector) SetLock(lockKey string, ttlSec int) (bool, error) {
	c := connector.pool.Get()
	defer c.Close()
	return connector.setLock(c, lockKey, ttlSec)
}

// SetTriggerCheckLock is the special case of SetLock for the given trigger id
func (connector *DbConnector) SetTriggerCheckLock(triggerID string) (bool, error) {
	return connector.SetLock(triggerLockKey(triggerID), moira.TriggerCheckLimit)
}

// SetTriggerCoolDown marks the fact that trigger has been processed by checker
// and there's no point to try to process it again
// unlike other lock methods, SetTriggerCoolDown doesn't have acquire or delete methods
func (connector *DbConnector) SetTriggerCoolDown(triggerID string, ttlSec int) (bool, error) {
	return connector.SetLock(triggerCoolDownKey(triggerID), ttlSec)
}

// DeleteLock deletes lock for given key
func (connector *DbConnector) DeleteLock(lockKey string) error {
	c := connector.pool.Get()
	defer c.Close()

	_, err := c.Do("DEL", lockKey)
	if err != nil {
		return fmt.Errorf("Failed to delete lock with key \"%s\", error: %s", lockKey, err.Error())
	} else {
		return nil
	}
}

// DeleteTriggerCheckLock is the special case of DeleteLock for the given triggerID
func (connector *DbConnector) DeleteTriggerCheckLock(triggerID string) error {
	return connector.DeleteLock(triggerLockKey(triggerID))
}

// DeleteTriggerMaintenanceLock is the special case of DeleteLock for the given triggerID
func (connector *DbConnector) DeleteTriggerMaintenanceLock(triggerID string) error {
	return connector.DeleteLock(triggerMaintenanceLockKey(triggerID))
}

func (connector *DbConnector) setLock(conn redis.Conn, lockKey string, ttlSec int) (bool, error) {
	_, err := redis.String(conn.Do("SET", lockKey, time.Now().Unix(), "EX", ttlSec, "NX"))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("Failed to set lock with key \"%s\", error: %s", lockKey, err.Error())
	}
	return true, nil
}

func triggerCoolDownKey(triggerID string) string {
	return fmt.Sprintf("moira-trigger-cool-down:%s", triggerID)
}

func triggerLockKey(triggerID string) string {
	return fmt.Sprintf("moira-metric-check-lock:%s", triggerID)
}

func triggerMaintenanceLockKey(triggerID string) string {
	return fmt.Sprintf("moira-trigger-maintenance-lock:%s", triggerID)
}
