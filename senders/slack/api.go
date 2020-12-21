package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"go.avito.ru/DO/moira/logging"
)

// Client is a Slack client with rsyslog.
type Client struct {
	*slack.Client
	logger *logging.Logger
}

func NewSlack(apiToken string, logger *logging.Logger) *Client {
	return &Client{
		Client: slack.New(apiToken),
		logger: logger,
	}
}

func (api *Client) UpdateMessage(channelId, messageTs, text string, escape bool) (string, string, string, error) {
	s1, s2, s3, err := api.Client.SendMessageContext(
		context.Background(),
		channelId,
		slack.MsgOptionUpdate(messageTs),
		slack.MsgOptionText(text, escape),
	)
	if err != nil {
		api.logger.ErrorE("slack: updating message failed", map[string]interface{}{
			"Channel": channelId,
			"TS":      messageTs,
			"Text":    text,
			"Escape":  escape,
			"Error":   err.Error(),
		})
	}
	return s1, s2, s3, err
}

func (api *Client) findAndJoinChannel(channelName string) (*slack.Channel, error) {
	if channelName[0] == '#' {
		channelName = channelName[1:]
	}
	getConversationParams := slack.GetConversationsParameters{
		Cursor:          "",
		Limit:           999,
		ExcludeArchived: "true",
	}
	for {
		channels, nextCursor, err := api.GetConversations(&getConversationParams)
		if err != nil {
			api.logger.ErrorE("slack: getting conversations failed", map[string]interface{}{
				"Error": err.Error(),
			})
			return nil, err
		}
		for _, channel := range channels {
			if channel.Name == channelName {
				newChannel, warning, warnings, err := api.JoinConversation(channel.ID)
				if warning != "" || len(warnings) > 0 {
					api.logger.WarnE("slack: received warning from conversations.join", map[string]interface{}{
						"Channel":     channel.ID,
						"ChannelName": channel.Name,
						"Warning":     warning,
						"Warnings":    warnings,
					})
				}
				if err != nil {
					api.logger.ErrorE("slack: joining channel failed", map[string]interface{}{
						"Channel":     channel.ID,
						"ChannelName": channel.Name,
						"Error":       err.Error(),
					})
					return nil, err
				}
				return newChannel, nil
			}
		}
		if nextCursor == "" {
			break
		} else {
			getConversationParams.Cursor = nextCursor
			time.Sleep(3 * time.Second)
		}
	}
	return nil, fmt.Errorf("channel %s not found", channelName)
}
