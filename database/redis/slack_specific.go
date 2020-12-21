package redis

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
)

const ThreadTTL = 30 * 24 * time.Hour

func (connector *DbConnector) GetSlackThreadLinks(contactID, triggerID string) (messages map[string]string, err error) {
	c := connector.pool.Get()
	defer c.Close()
	key := messageIDKey(contactID, triggerID)

	messages = make(map[string]string)
	response, err := redis.Strings(c.Do("HGETALL", key))
	if err != nil {
		if err == redis.ErrNil {
			return messages, database.ErrNil
		} else {
			return messages, err
		}
	}

	for i := 0; i < len(response); i += 2 {
		dashboardTs := response[i]
		threadTs := response[i+1]
		messages[threadTs] = dashboardTs
	}

	return messages, err
}

func (connector *DbConnector) GetAllSlackThreadLinks(triggerID string) ([]moira.SlackThreadLink, error) {
	c := connector.pool.Get()
	defer c.Close()
	key := allMessageIDsKey(triggerID)

	response, err := redis.Strings(c.Do("SMEMBERS", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil, database.ErrNil
		} else {
			return nil, err
		}
	}

	result := make([]moira.SlackThreadLink, len(response))
	for i, item := range response {
		split := strings.SplitN(item, ":", 3)
		if len(split) < 3 {
			return nil, fmt.Errorf("failed to parse %s as SlackThreadLink", item)
		}
		result[i] = moira.SlackThreadLink{
			Contact:     split[0],
			ThreadTs:    split[1],
			DashboardTs: split[2],
		}
	}
	return result, nil
}

func (connector *DbConnector) AddSlackThreadLinks(contactID, triggerID, dashboardTs, threadTs string, expiryTime *time.Time) error {
	c := connector.pool.Get()
	defer c.Close()
	key := messageIDKey(contactID, triggerID)
	keyAll := allMessageIDsKey(triggerID)

	if expiryTime == nil {
		t := time.Now().Add(ThreadTTL)
		expiryTime = &t
	}

	c.Send("MULTI")
	c.Send("HSET", key, dashboardTs, threadTs)
	c.Send("SADD", keyAll, linkStorageKey(contactID, threadTs, dashboardTs))
	c.Send("EXPIREAT", key, expiryTime.Unix())
	if _, err := c.Do("EXEC"); err != nil {
		return fmt.Errorf(
			"Failed to add sender message ID to Redis, messenger: slack, contact: %s, trigger: %s, error: %s",
			contactID, triggerID,
			err.Error(),
		)
	}

	return nil
}

func (connector *DbConnector) RemoveSlackThreadLinks(contactID, triggerID string, dashboardsTs, threadsTs []string) error {
	c := connector.pool.Get()
	defer c.Close()

	// `c.Do` thinks that []string is not []interface{}
	// so we need an explicit conversion
	delArgs := make([]interface{}, len(dashboardsTs)+1)
	delAllArgs := make([]interface{}, len(dashboardsTs)+1)
	delArgs[0] = messageIDKey(contactID, triggerID)
	delAllArgs[0] = allMessageIDsKey(triggerID)
	for i, _ := range dashboardsTs {
		delArgs[i+1] = dashboardsTs[i]
		delAllArgs[i+1] = linkStorageKey(contactID, threadsTs[i], dashboardsTs[i])
	}

	c.Send("MULTI")
	c.Send("HDEL", delArgs...)
	c.Send("SREM", delAllArgs...)
	if _, err := c.Do("EXEC"); err != nil {
		return fmt.Errorf(
			"Failed to remove sender message IDs from Redis, messenger: slack, contact: %s, trigger: %s, error: %s",
			contactID, triggerID,
			err.Error(),
		)
	}
	return nil
}

func messageIDKey(contactID, triggerID string) string {
	return fmt.Sprintf("moira-sender-slack-threads:%s:%s", contactID, triggerID)
}

func allMessageIDsKey(triggerID string) string {
	return fmt.Sprintf("moira-sender-slack-all-threads:%s", triggerID)
}

func linkStorageKey(contact, threadTs, dashboardTs string) string {
	return fmt.Sprintf("%s:%s:%s", contact, threadTs, dashboardTs)
}

func getDashboardFromRedis(data []byte) (moira.SlackDashboard, error) {
	result := make(moira.SlackDashboard)
	err := json.Unmarshal(data, &result)
	return result, err
}

func renderForRedis(sb moira.SlackDashboard) []byte {
	jsonData, _ := json.Marshal(sb)
	return jsonData
}

func (connector *DbConnector) GetSlackDashboard(contactID, ts string) (moira.SlackDashboard, error) {
	c := connector.pool.Get()
	defer c.Close()

	key := slackDashboardKey(contactID, ts)
	response, err := redis.Bytes(c.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return moira.SlackDashboard{}, database.ErrNil
		} else {
			return moira.SlackDashboard{}, err
		}
	}

	result, err := getDashboardFromRedis(response)
	return result, err
}

func (connector *DbConnector) UpdateSlackDashboard(contactID, ts string, db moira.SlackDashboard, expiryTime *time.Time) error {
	c := connector.pool.Get()
	defer c.Close()
	key := slackDashboardKey(contactID, ts)

	if expiryTime == nil {
		t := time.Now().Add(ThreadTTL)
		expiryTime = &t
	}

	c.Send("SET", key, renderForRedis(db))
	c.Send("EXPIREAT", key, expiryTime.Unix())
	if _, err := c.Do("EXEC"); err != nil {
		return fmt.Errorf(
			"Failed to save Slack dashboard to Redis, ts: %s, error: %s",
			ts,
			err.Error(),
		)
	}

	return nil
}

func (connector *DbConnector) RemoveSlackDashboards(contactID string, dashboardsTs []string) error {
	c := connector.pool.Get()
	defer c.Close()
	keys := make([]interface{}, len(dashboardsTs))
	for i, ts := range dashboardsTs {
		keys[i] = slackDashboardKey(contactID, ts)
	}

	_, err := c.Do("DEL", keys...)
	if err != nil {
		return fmt.Errorf(
			"Failed to remove Slack dashboards from Redis, ts %v, error: %s",
			dashboardsTs,
			err.Error(),
		)
	}
	return nil
}

func slackDashboardKey(contactID, ts string) string {
	return fmt.Sprintf("moira-sender-slack-dashboard:%s:%s", contactID, ts)
}

func (connector *DbConnector) GetAllInheritedTriggerDashboards(triggerID, ancestorTriggerID, ancestorMetric string) ([]moira.SlackThreadLink, error) {
	c := connector.pool.Get()
	defer c.Close()
	key := slackInheritedDashboardKey(triggerID, ancestorTriggerID, ancestorMetric)

	response, err := redis.Strings(c.Do("SMEMBERS", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil, database.ErrNil
		} else {
			return nil, err
		}
	}

	result := make([]moira.SlackThreadLink, len(response))
	for i, item := range response {
		split := strings.SplitN(item, ":", 3)
		if len(split) < 3 {
			return nil, fmt.Errorf("failed to parse %s as SlackThreadLink", item)
		}
		result[i] = moira.SlackThreadLink{
			Contact:     split[0],
			ThreadTs:    split[1],
			DashboardTs: split[2],
		}
	}

	return result, nil
}

func (connector *DbConnector) SaveInheritedTriggerDashboard(
	contactID, threadTs,
	triggerID, ancestorTriggerID, ancestorMetric,
	newDashboardTs string,
) error {
	c := connector.pool.Get()
	defer c.Close()
	key := slackInheritedDashboardKey(triggerID, ancestorTriggerID, ancestorMetric)

	c.Send("SADD", key, linkStorageKey(contactID, threadTs, newDashboardTs))
	c.Send("EXPIREAT", key, time.Now().Add(ThreadTTL).Unix())

	if _, err := c.Do("EXEC"); err != nil {
		return fmt.Errorf(
			"Failed to save Slack inherited dashboard to Redis, ts: %s, error: %s",
			newDashboardTs,
			err.Error(),
		)
	}
	return nil
}

func (connector *DbConnector) DeleteInheritedTriggerDashboard(
	contactID, threadTs,
	triggerID, ancestorTriggerID, ancestorMetric,
	dashboardTs string,
) error {
	c := connector.pool.Get()
	defer c.Close()
	key := slackInheritedDashboardKey(triggerID, ancestorTriggerID, ancestorMetric)

	_, err := c.Do("SREM", key, linkStorageKey(contactID, threadTs, dashboardTs))
	if err != nil {
		return fmt.Errorf(
			"Failed to delete Slack inherited dashboard from Redis, trigger: %s, error: %s",
			triggerID,
			err.Error(),
		)
	}
	return nil
}

func slackInheritedDashboardKey(triggerID, ancestorTriggerID, ancestorMetric string) string {
	return fmt.Sprintf("moira-sender-slack-inherited-dashboards:%s:%s:%s", triggerID, ancestorTriggerID, ancestorMetric)
}

func (connector *DbConnector) GetServiceDuty(service string) (moira.DutyData, error) {
	c := connector.pool.Get()
	defer c.Close()
	key := dutyKey(service)
	var result moira.DutyData

	response, err := redis.Bytes(c.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return result, database.ErrNil
		} else {
			return result, err
		}
	}

	err = json.Unmarshal(response, &result)
	if err != nil {
		return result, fmt.Errorf("Failed to parse duty json %s: %s", string(response), err.Error())
	}

	return result, nil
}

func (connector *DbConnector) UpdateServiceDuty(service string, dutyData moira.DutyData) error {
	c := connector.pool.Get()
	defer c.Close()

	key := dutyKey(service)
	raw, err := json.Marshal(dutyData)
	if err != nil {
		return err
	}
	_, err = c.Do("SET", key, raw)
	if err != nil {
		return fmt.Errorf(
			"Failed to save duty data to Redis, service %s, error: %s",
			service,
			err.Error(),
		)
	}

	return nil
}

func dutyKey(service string) string {
	return fmt.Sprintf("moira-service-duty:%s", service)
}

func (connector *DbConnector) FetchSlackDelayedActions(until time.Time) ([]moira.SlackDelayedAction, error) {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("ZRANGEBYSCORE", apiActionKey, "-inf", until.Unix())
	c.Send("ZREMRANGEBYSCORE", apiActionKey, "-inf", until.Unix())
	rawRes, err := c.Do("EXEC")
	if rawRes == nil {
		return []moira.SlackDelayedAction{}, database.ErrNil
	}

	values, err := redis.Values(rawRes, err)
	if err != nil {
		return []moira.SlackDelayedAction{}, fmt.Errorf("Failed to fetch Slack API action: %s", err.Error())
	}

	res, err := redis.ByteSlices(values[0], nil)
	if err != nil {
		return []moira.SlackDelayedAction{}, fmt.Errorf("Failed to parse Slack API action: %s", err.Error())
	}

	result := make([]moira.SlackDelayedAction, 0, len(res))
	for _, actionRaw := range res {
		var act moira.SlackDelayedAction
		if err := json.Unmarshal(actionRaw, &act); err != nil {
			return []moira.SlackDelayedAction{}, fmt.Errorf("Failed to parse Slack API action from JSON: %s", err.Error())
		}
		result = append(result, act)
	}

	return result, nil
}

func (connector *DbConnector) SaveSlackDelayedAction(action moira.SlackDelayedAction) error {
	c := connector.pool.Get()
	defer c.Close()

	raw, err := json.Marshal(action)
	if err != nil {
		return err
	}
	_, err = c.Do("ZADD", apiActionKey, action.ScheduledAt.Unix(), raw)
	if err != nil {
		return fmt.Errorf(
			"Failed to save Slack API action [%+v] to Redis: %s",
			action,
			err.Error(),
		)
	}

	return nil
}

func (connector *DbConnector) GetSlackUserGroups() (moira.SlackUserGroupsCache, error) {
	c := connector.pool.Get()
	defer c.Close()

	data, err := redis.Bytes(c.Do("GET", userGroupsKey))
	if err != nil {
		if err == redis.ErrNil {
			err = database.ErrNil
		}
		return nil, err
	}

	result := make(moira.SlackUserGroupsCache)
	err = json.Unmarshal(data, &result)
	return result, err
}

func (connector *DbConnector) SaveSlackUserGroups(userGroups moira.SlackUserGroupsCache) error {
	c := connector.pool.Get()
	defer c.Close()

	data, err := json.Marshal(userGroups)
	if err != nil {
		return err
	}

	_, err = c.Do("SET", userGroupsKey, data)
	if err != nil {
		err = fmt.Errorf("Failed to save slack user groups: %v", err)
	}
	return err
}

const (
	apiActionKey  = "moira-slack-api-actions"
	userGroupsKey = "moira-slack-user-groups"
)
