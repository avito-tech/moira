package redis

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/database/redis/reply"
)

// GetMaintenanceSilent is convenience method for getting maintenance structure for the given silent pattern type
func (connector *DbConnector) GetMaintenanceSilent(spt moira.SilentPatternType) (moira.Maintenance, error) {
	c := connector.pool.Get()
	defer c.Close()

	return connector.getMaintenance(c, maintenanceKeySilent(spt))
}

// GetMaintenanceTrigger is convenience method for getting trigger's maintenance
func (connector *DbConnector) GetMaintenanceTrigger(id string) (moira.Maintenance, error) {
	c := connector.pool.Get()
	defer c.Close()

	return connector.getMaintenance(c, maintenanceKeyTrigger(id))
}

// GetOrCreateMaintenanceSilent does the same that GetMaintenanceSilent does
// but ignores not existing error and returns an empty structure in this case
func (connector *DbConnector) GetOrCreateMaintenanceSilent(spt moira.SilentPatternType) (moira.Maintenance, error) {
	c := connector.pool.Get()
	defer c.Close()

	return connector.getOrCreateMaintenanceSilent(c, spt)
}

// GetOrCreateMaintenanceTrigger does the same that GetMaintenanceTrigger does
// but ignores not existing error and returns an empty structure in this case
func (connector *DbConnector) GetOrCreateMaintenanceTrigger(id string) (moira.Maintenance, error) {
	c := connector.pool.Get()
	defer c.Close()

	maintenance, err := connector.getMaintenance(c, maintenanceKeyTrigger(id))
	if err == database.ErrNil {
		err = nil
	}
	return maintenance, err
}

// SetMaintenanceTrigger stores current maintenance of the trigger to db
// note! if maintenance of some metric or the whole trigger is deleted,
// then SetMaintenanceTrigger should be used to save the new value
func (connector *DbConnector) SetMaintenanceTrigger(id string, maintenance moira.Maintenance) error {
	c := connector.pool.Get()
	defer c.Close()

	return connector.setMaintenanceTrigger(c, id, maintenance, true)
}

// DelMaintenanceTrigger deletes current maintenance data of the trigger
// it should be used only when the trigger is deleted
func (connector *DbConnector) DelMaintenanceTrigger(id string) error {
	c := connector.pool.Get()
	defer c.Close()

	return connector.delMaintenanceTrigger(c, id, true)
}

func (connector *DbConnector) getMaintenance(conn redis.Conn, key string) (moira.Maintenance, error) {
	maintenance, err := reply.Maintenance(conn.Do("GET", key))
	if err == nil {
		maintenance.Clean()
	}
	return maintenance, err
}

func (connector *DbConnector) getOrCreateMaintenanceSilent(conn redis.Conn, spt moira.SilentPatternType) (moira.Maintenance, error) {
	maintenance, err := connector.getMaintenance(conn, maintenanceKeySilent(spt))
	if err == database.ErrNil {
		err = nil
	}
	return maintenance, err
}

func (connector *DbConnector) setMaintenance(conn redis.Conn, key string, maintenance moira.Maintenance, eager bool) error {
	bytes, err := json.Marshal(maintenance)
	if err != nil {
		return errors.Wrap(err, "failed to marshal")
	}

	if eager {
		_, err = conn.Do("SET", key, bytes)
	} else {
		err = conn.Send("SET", key, bytes)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to SET key %s", key)
	}

	return nil
}

func (connector *DbConnector) setMaintenanceSilent(conn redis.Conn, spt moira.SilentPatternType, maintenance moira.Maintenance, eager bool) error {
	return connector.setMaintenance(conn, maintenanceKeySilent(spt), maintenance, eager)
}

func (connector *DbConnector) setMaintenanceTrigger(conn redis.Conn, id string, maintenance moira.Maintenance, eager bool) error {
	return connector.setMaintenance(conn, maintenanceKeyTrigger(id), maintenance, eager)
}

func (connector *DbConnector) delMaintenance(conn redis.Conn, key string, eager bool) error {
	var (
		err error
	)

	if eager {
		_, err = conn.Do("DEL", key)
	} else {
		err = conn.Send("DEL", key)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to DEL key %s", key)
	}

	return nil
}

func (connector *DbConnector) delMaintenanceSilent(conn redis.Conn, spt moira.SilentPatternType) error {
	return connector.delMaintenance(conn, maintenanceKeySilent(spt), true)
}

func (connector *DbConnector) delMaintenanceTrigger(conn redis.Conn, id string, eager bool) error {
	return connector.delMaintenance(conn, maintenanceKeyTrigger(id), eager)
}

func maintenanceKeyTrigger(id string) string {
	return fmt.Sprintf("moira-maintenance-trigger:%s", id)
}

func maintenanceKeySilent(spt moira.SilentPatternType) string {
	return fmt.Sprintf("moira-maintenance-silent:%d", spt)
}
