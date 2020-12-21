package worker

import (
	"fmt"
	"runtime/debug"
	"time"
)

func (worker *Checker) tagsChecker() error {
	checkTicker := time.NewTicker(worker.Config.TagsCheckInterval)
	for {
		select {
		case <-worker.tomb.Dying():
			checkTicker.Stop()
			worker.Logger.Info("Empty tags checker stopped")
			close(worker.tagsToCheck)
			return nil
		case <-checkTicker.C:
			if tags, err := worker.Database.GetTagNames(); err != nil {
				worker.Logger.ErrorF("Empty tags check failed: %s", err.Error())
			} else {
				for _, tag := range tags {
					worker.tagsToCheck <- tag
				}
			}
		}
	}
}

func (worker *Checker) tagsHandler(ch <-chan string) error {
	for {
		if tag, ok := <-ch; !ok {
			return nil
		} else {
			worker.handleSingleTag(tag)
		}
	}
}

func (worker *Checker) handleSingleTag(tag string) {
	var err error
	defer func() {
		if r := recover(); r != nil || err != nil {
			worker.Logger.ErrorE("Error or panic while handling single tag", map[string]interface{}{
				"error": fmt.Sprintf("%#v", err),
				"panic": fmt.Sprintf("%#v", r),
				"stack": string(debug.Stack()),
				"tag":   tag,
			})
		}
	}()

	// There should be no triggers corresponding to this tag
	triggerIDs, err := worker.Database.GetTriggerCheckIDs([]string{tag}, false)
	if len(triggerIDs) > 0 || err != nil {
		return
	}

	// And no subscriptions as well
	subscriptions, err := worker.Database.GetTagsSubscriptions([]string{tag})
	if len(subscriptions) > 0 || err != nil {
		return
	}

	err = worker.Database.RemoveTag(tag)
	if err == nil {
		worker.Logger.InfoE("Empty tag has been successfully removed", []string{tag})
	}
}
