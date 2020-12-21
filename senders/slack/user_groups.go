package slack

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"
	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
)

const (
	checkInterval = 5 * time.Minute
)

type UserGroupsWorker struct {
	apiToken string
	client   *slack.Client
	db       moira.Database
	logger   moira.Logger
	tomb     tomb.Tomb
}

func NewUserGroupsWorker(apiToken string, db moira.Database, logger moira.Logger) *UserGroupsWorker {
	return &UserGroupsWorker{
		apiToken: apiToken,
		client:   slack.New(apiToken),
		db:       db,
		logger:   logger,
	}
}

func (worker *UserGroupsWorker) Start() {
	ticker := time.NewTicker(checkInterval)
	worker.tomb.Go(func() error {
		worker.logger.Info("Moira Notifier Slack UserGroupsWorker started.")
		worker.refreshUserGroups()

		for {
			select {
			case <-worker.tomb.Dying():
				worker.logger.Info("Moira Notifier Slack UserGroupsWorker stopped.")
				return nil
			case <-ticker.C:
				worker.refreshUserGroups()
			}
		}
	})
}

func (worker *UserGroupsWorker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}

func (worker *UserGroupsWorker) refreshUserGroups() {
	worker.logger.Info("Refreshing user groups cache.")

	groups, err := worker.client.GetUserGroups(slack.GetUserGroupsOptionIncludeUsers(true))
	if err != nil {
		worker.logger.ErrorF("Failed to get groups: %v.", err)
		return
	}
	worker.traceUserGroups(groups)

	badGroups := make(map[string]bool)
	userGroupsCache := make(moira.SlackUserGroupsCache, len(groups))

	for _, group := range groups {
		if !group.IsUserGroup || group.DateDelete != 0 || group.Handle == "" {
			// skip such "groups", just in case (there shouldn't be such groups though)
			continue
		}

		if _, ok := userGroupsCache[group.Handle]; ok {
			worker.logger.WarnF("Found duplicated group: %s. It won't be included to result.", group.Handle)

			badGroups[group.Handle] = true
			continue
		}

		userGroupsCache[group.Handle] = moira.SlackUserGroup{
			Id:         group.ID,
			Handle:     group.Handle,
			Name:       group.Name,
			DateCreate: group.DateCreate.Time(),
			DateUpdate: group.DateUpdate.Time(),
			UserIds:    append([]string(nil), group.Users...),
		}
	}

	for badGroup := range badGroups {
		delete(userGroupsCache, badGroup)
	}

	err = worker.db.SaveSlackUserGroups(userGroupsCache)
	worker.logger.InfoF("Saved user groups cache, err = %v.", err)
}

func (worker *UserGroupsWorker) traceUserGroups(groups []slack.UserGroup) {
	const chunkSize = 50

	groupsQty := len(groups)
	worker.logger.InfoF("Successfully got %d groups from slack.", groupsQty)

	for i := 0; i < groupsQty; i += chunkSize {
		lo := i
		hi := i + chunkSize
		if hi > groupsQty {
			hi = groupsQty
		}

		worker.logger.InfoE(fmt.Sprintf("Groups chunk #%d", (i/chunkSize)+1), groups[lo:hi])
	}
}
