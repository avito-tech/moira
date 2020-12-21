package redis

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/satori/go.uuid"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/database/redis/reply"
)

// GetSubscription returns subscription data by given id, if no value, return database.ErrNil error
func (connector *DbConnector) GetSubscription(id string) (moira.SubscriptionData, error) {
	c := connector.pool.Get()
	defer c.Close()

	subscription, err := reply.Subscription(c.Do("GET", subscriptionKey(id)))
	if err != nil {
		return subscription, err
	}
	subscription.ID = id
	return subscription, nil
}

// GetSubscriptions returns subscriptions data by given ids, len of subIds is equal to len of returned values array.
// If there is no object by current ID, then nil is returned
func (connector *DbConnector) GetSubscriptions(subIds []string) ([]*moira.SubscriptionData, error) {
	c := connector.pool.Get()
	defer c.Close()
	return connector.getSubscriptions(c, subIds)
}

// getSubscriptions does all the logic of GetSubscriptions with reusing of redis connection
func (connector *DbConnector) getSubscriptions(conn redis.Conn, subIds []string) ([]*moira.SubscriptionData, error) {
	_ = conn.Send("MULTI")
	for _, id := range subIds {
		_ = conn.Send("GET", subscriptionKey(id))
	}

	data, err := reply.Subscriptions(conn.Do("EXEC"))
	if err != nil {
		return nil, fmt.Errorf("Failed to EXEC: %v", err)
	}

	for i := range data {
		if data[i] != nil {
			data[i].ID = subIds[i]
		}
	}
	return data, nil
}

// GetAllSubscriptions returns all existing subscriptions
func (connector *DbConnector) GetAllSubscriptions() ([]*moira.SubscriptionData, error) {
	c := connector.pool.Get()
	defer c.Close()

	keys, err := redis.Strings(c.Do("KEYS", subscriptionKey("*")))
	if err != nil {
		return nil, err
	}

	subscriptionIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		key = key[19:]
		subscriptionIDs = append(subscriptionIDs, key)
	}

	return connector.GetSubscriptions(subscriptionIDs)
}

// MaybeUpdateEscalationsForSubscription provides primary key (ID) to Escalations of this subscription
func (connector *DbConnector) MaybeUpdateEscalationsOfSubscription(subscription *moira.SubscriptionData) error {
	// check whether at least one escalation has no ID, to avoid redundant DB queries
	hasEscalationsWoID := false
	for _, escalation := range subscription.Escalations {
		if escalation.ID == "" {
			hasEscalationsWoID = true
			break
		}
	}

	if hasEscalationsWoID {
		return connector.SaveSubscription(subscription)
	} else {
		return nil
	}
}

// SaveSubscription writes subscription data, updates tags subscriptions and user subscriptions
func (connector *DbConnector) SaveSubscription(subscription *moira.SubscriptionData) error {
	oldSubscription, getSubError := connector.GetSubscription(subscription.ID)
	if getSubError != nil && getSubError != database.ErrNil {
		return getSubError
	}
	c := connector.pool.Get()
	defer c.Close()
	c.Send("MULTI")
	if getSubError != database.ErrNil {
		addSendSubscriptionRequest(c, subscription, &oldSubscription)
	} else {
		addSendSubscriptionRequest(c, subscription, nil)
	}
	_, err := c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	return nil
}

// SaveSubscriptions writes subscriptions, updates tags subscriptions and user subscriptions
func (connector *DbConnector) SaveSubscriptions(subscriptions []*moira.SubscriptionData) error {
	ids := make([]string, len(subscriptions))
	for i, subscription := range subscriptions {
		ids[i] = subscription.ID
	}
	oldSubscriptions, err := connector.GetSubscriptions(ids)
	if err != nil {
		return err
	}
	c := connector.pool.Get()
	defer c.Close()
	c.Send("MULTI")
	for i, subscription := range subscriptions {
		addSendSubscriptionRequest(c, subscription, oldSubscriptions[i])
	}
	_, err = c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	return nil
}

// RemoveSubscription deletes subscription data and removes subscriptionID from users and tags subscriptions
func (connector *DbConnector) RemoveSubscription(subscriptionID string) error {
	subscription, err := connector.GetSubscription(subscriptionID)
	if err != nil {
		if err == database.ErrNil {
			return nil
		}
		return err
	}
	c := connector.pool.Get()
	defer c.Close()
	c.Send("MULTI")
	c.Send("SREM", userSubscriptionsKey(subscription.User), subscriptionID)
	for _, tag := range subscription.Tags {
		c.Send("SREM", tagSubscriptionKey(tag), subscriptionID)
	}
	c.Send("DEL", subscriptionKey(subscription.ID))
	_, err = c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	return nil
}

// GetUserSubscriptionIDs returns subscriptions ids by given login
func (connector *DbConnector) GetUserSubscriptionIDs(login string) ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()

	subscriptions, err := redis.Strings(c.Do("SMEMBERS", userSubscriptionsKey(login)))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve subscriptions for user login %s: %s", login, err.Error())
	}
	return subscriptions, nil
}

// GetTagsSubscriptions gets all subscriptionsIDs by given tag list and read subscriptions.
// Len of subscriptionIDs is equal to len of returned values array. If there is no object by current ID, then nil is returned
func (connector *DbConnector) GetTagsSubscriptions(tags []string) ([]*moira.SubscriptionData, error) {
	c := connector.pool.Get()
	defer c.Close()
	return connector.getTagsSubscriptions(c, tags)
}

// getTagsSubscriptions does all the logic of GetTagsSubscriptions with reusing of redis connection
func (connector *DbConnector) getTagsSubscriptions(conn redis.Conn, tags []string) ([]*moira.SubscriptionData, error) {
	keys := convertArgs(tags, func(tag string) string {
		return fmt.Sprintf("moira-tag-subscriptions:%s", tag)
	})

	values, err := redis.Values(conn.Do("SUNION", keys...))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve subscriptions for tags %v: %v", tags, err)
	}

	var subIds []string
	if err := redis.ScanSlice(values, &subIds); err != nil {
		return nil, fmt.Errorf("Failed to retrieve subscriptions for tags %v: %v", tags, err)
	}

	if len(subIds) == 0 {
		return make([]*moira.SubscriptionData, 0), nil
	}
	return connector.getSubscriptions(conn, subIds)
}

func addSendSubscriptionRequest(c redis.Conn, subscription *moira.SubscriptionData, oldSubscription *moira.SubscriptionData) error {
	for i := range subscription.Escalations {
		if subscription.Escalations[i].ID == "" {
			subscription.Escalations[i].ID = uuid.NewV4().String()
		}
	}

	bytes, err := json.Marshal(subscription)
	if err != nil {
		return err
	}
	if oldSubscription != nil {
		for _, tag := range oldSubscription.Tags {
			c.Send("SREM", tagSubscriptionKey(tag), subscription.ID)
		}
		if oldSubscription.User != subscription.User {
			c.Send("SREM", userSubscriptionsKey(oldSubscription.User), subscription.ID)
		}
	}
	for _, tag := range subscription.Tags {
		c.Send("SADD", tagSubscriptionKey(tag), subscription.ID)
	}
	c.Send("SADD", userSubscriptionsKey(subscription.User), subscription.ID)
	c.Send("SET", subscriptionKey(subscription.ID), bytes)
	return nil
}

func subscriptionKey(id string) string {
	return fmt.Sprintf("moira-subscription:%s", id)
}

func userSubscriptionsKey(userName string) string {
	return fmt.Sprintf("moira-user-subscriptions:%s", userName)
}
