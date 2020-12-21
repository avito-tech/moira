package redis

import (
	"fmt"

	"github.com/garyburd/redigo/redis"

	"go.avito.ru/DO/moira"
)

const (
	tagsKey = "moira-tags"
)

// GetTagNames returns all tags from set with tag data
func (connector *DbConnector) GetTagNames() ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()

	tagNames, err := redis.Strings(c.Do("SMEMBERS", tagsKey))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve tags: %s", err.Error())
	}
	return tagNames, nil
}

// RemoveTag deletes tag from tags list, deletes triggerIDs and subscriptionsIDs lists by given tag
func (connector *DbConnector) RemoveTag(tagName string) error {
	c := connector.pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("SREM", tagsKey, tagName)
	c.Send("DEL", tagSubscriptionKey(tagName))
	c.Send("DEL", tagTriggersKey(tagName))
	_, err := c.Do("EXEC")
	if err != nil {
		return fmt.Errorf("Failed to EXEC: %s", err.Error())
	}
	return nil
}

// GetTagTriggerIDs gets all triggersIDs by given tag
func (connector *DbConnector) GetTagTriggerIDs(tag string) ([]string, error) {
	c := connector.pool.Get()
	defer c.Close()
	return connector.getTagTriggersIDs(c, tag)
}

// getTagTriggersIDs does all the logic of GetTagTriggerIDs with reusing of redis connection
func (connector *DbConnector) getTagTriggersIDs(conn redis.Conn, tag string) ([]string, error) {
	ids, err := redis.Strings(conn.Do("SMEMBERS", tagTriggersKey(tag)))
	if err == redis.ErrNil {
		return make([]string, 0), nil
	}
	if err == nil {
		return ids, nil
	}
	return nil, fmt.Errorf("Failed to retrieve tag triggers:%s, err: %v", tag, err)
}

// GetTagsStats collects moira.TagStats data for each tag given
// tags are assumed to be unique
func (connector *DbConnector) GetTagsStats(tags ...string) ([]moira.TagStats, error) {
	qty := len(tags)
	if qty == 0 {
		return nil, nil
	}

	c := connector.pool.Get()
	defer c.Close()

	// request subscriptions for all tags
	subscriptions, err := connector.getTagsSubscriptions(c, tags)
	if err != nil {
		return nil, err
	}

	// maps tag to its index in the source list
	positions := make(map[string]int, qty)
	// trigger ids for each tag will be requested concurrently
	src := make(chan string, qty)

	for i, tag := range tags {
		positions[tag] = i
		src <- tag
	}
	close(src)

	// starting concurrent trigger ids collection
	// some data might be missing due to errors
	dst := make(chan []string, qty)
	for i := 0; i < 192; i++ {
		go func(src chan string, dst chan []string) {
			var (
				conn redis.Conn
			)

			for tag := range src {
				if conn == nil {
					conn = connector.pool.Get()
				}

				ids, err := connector.getTagTriggersIDs(conn, tag)
				if err != nil {
					dst <- nil
				} else {
					dst <- append([]string{tag}, ids...)
				}
			}

			if conn != nil {
				_ = conn.Close()
			}
		}(src, dst)
	}

	// spread subscriptions to their relevant tags
	result := make([]moira.TagStats, qty)
	for _, sub := range subscriptions {
		if sub == nil {
			continue
		}

		for _, tag := range sub.Tags {
			if pos, ok := positions[tag]; ok {
				result[pos].Subscriptions = append(result[pos].Subscriptions, *sub)
			}
		}
	}

	// spread trigger ids to their relevant tags
	for i := 0; i < qty; i++ {
		if data := <-dst; data != nil {
			tag, ids := data[0], data[1:]
			if pos, ok := positions[tag]; ok {
				result[pos].Triggers = ids
			}
		}
		result[i].Name = tags[i]
	}

	return result, nil
}

func tagTriggersKey(tagName string) string {
	return fmt.Sprintf("moira-tag-triggers:%s", tagName)
}

func tagSubscriptionKey(tagName string) string {
	return fmt.Sprintf("moira-tag-subscriptions:%s", tagName)
}
