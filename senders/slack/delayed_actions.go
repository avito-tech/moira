package slack

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/slack-go/slack"
	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/metrics"
)

const DelayedActionWorkerId = "slack-delayed"

type DelayedActionWorker struct {
	APIToken string
	DataBase moira.Database
	logger   moira.Logger
	tomb     tomb.Tomb
}

func (worker *DelayedActionWorker) Start() {
	notifierMetrics := metrics.NewNotifierMetrics()
	worker.tomb.Go(func() error {
		for {
			select {
			case <-worker.tomb.Dying():
				worker.logger.Info("Moira Notifier Fetching Slack delayed actions stopped")
				return nil
			default:
				// pass
			}

			actions, err := worker.DataBase.FetchSlackDelayedActions(time.Now())
			if err != nil {
				if err != database.ErrNil {
					notifierMetrics.EventsMalformed.Increment()
					worker.logger.WarnF("Failed to fetch slack delayed reactions: %s", err.Error())
				}
				continue
			}
			notifierMetrics.EventsReceived.Count(len(actions))

			for _, action := range actions {
				err := worker.performAction(action)
				if err == nil {
					if metric, ok := notifierMetrics.SendersOkMetrics.GetMetric(DelayedActionWorkerId); ok {
						metric.Increment()
					}
					worker.logger.InfoE("Successfully performed Slack delayed action", action)
					continue
				}

				action.ScheduledAt = time.Now().Add(worker.calculateBackoff(action.FailCount))
				action.FailCount += 1
				_ = worker.DataBase.SaveSlackDelayedAction(action)

				if metric, ok := notifierMetrics.SendersFailedMetrics.GetMetric(DelayedActionWorkerId); ok {
					metric.Increment()
				}
				notifierMetrics.EventsProcessingFailed.Increment()
				worker.logger.ErrorE(
					fmt.Sprintf("Error while performing Slack delayed action: %v", err),
					map[string]interface{}{
						"action": action,
						"error":  err.Error(),
					},
				)
				time.Sleep(time.Second * 2)
			}
		}
	})
	worker.logger.Info("Moira Notifier Fetching Slack delayed actions started")
}

func (worker *DelayedActionWorker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}

func (worker *DelayedActionWorker) addReaction(itemRef slack.ItemRef, action *moira.SlackDelayedAction) error {
	api := slack.New(worker.APIToken)

	if err := api.AddReaction("jr-approve", itemRef); err != nil {
		switch err.Error() {

		case "already_reacted":
			// pass

		case "no_item_specified", "bad_timestamp":
			worker.logger.ErrorF(
				"Failed to send reaction to slack [%s:%s], will not retry: %s",
				action.Contact.Value,
				itemRef.Timestamp,
				err.Error(),
			)

		case "channel_not_found", "message_not_found":
			worker.logger.WarnF(
				"Failed to send reaction to slack [%s:%s], will not retry: %s",
				action.Contact.Value,
				itemRef.Timestamp,
				err.Error(),
			)

		default:
			return err
		}
	}

	return nil
}

func (worker *DelayedActionWorker) calculateBackoff(failCount int) time.Duration {
	if failCount < 5 {
		return time.Duration(math.Pow(4, float64(failCount))) * time.Second
	} else {
		return 15 * time.Minute
	}
}

func (worker *DelayedActionWorker) performAction(action moira.SlackDelayedAction) error {
	switch action.Action {

	case "AddReaction":
		var itemRef slack.ItemRef
		if err := json.Unmarshal(action.EncodedArgs, &itemRef); err != nil {
			return err
		}
		return worker.addReaction(itemRef, &action)

	default:
		return fmt.Errorf("unknown Slack API action: %s", action.Action)

	}
}
