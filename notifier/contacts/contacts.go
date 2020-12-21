package contacts

import (
	"strings"
	"time"

	"go.avito.ru/DO/moira"
)

type ContactReplacement struct {
	Expiration    *time.Time
	ValueReplaced string
	ValueRollback string
}

type Decoder struct {
	db           moira.Database
	logger       moira.Logger
	dutyAPIToken string
	dutyUrl      string
}

func NewDecoder(db moira.Database, logger moira.Logger, dutyAPIToken, dutyUrl string) *Decoder {
	var (
		err    error = nil
		result       = &Decoder{db: db, logger: logger}
	)

	result.dutyAPIToken, err = moira.GetFileContent(dutyAPIToken)
	result.dutyAPIToken = strings.TrimSpace(result.dutyAPIToken)
	if err != nil || result.dutyAPIToken == "" {
		logger.WarnF("Failed to read duty.avito.ru token from path \"%s\", or it is empty. Error is %v.", dutyAPIToken, err)
	}

	result.dutyUrl = dutyUrl
	if result.dutyUrl == "" {
		result.dutyUrl = "https://duty.avito.ru" // default
	}

	return result
}

const (
	deployer            = "_deployer"
	dbaasServiceChannel = "_dbaas_service_channel"
	dutyPrefix          = "duty_"
	groupPrefix         = "group__"
)

// UnwrapContact processes moira.ContactData using its value as template and moira.NotificationEvents as context
func (decoder *Decoder) UnwrapContact(contact *moira.ContactData, events moira.NotificationEvents) ([]ContactReplacement, error) {
	if contact.Type == "slack" {
		if strings.HasPrefix(contact.Value, dutyPrefix) {
			return decoder.unwrapDuty(contact, events)
		}
		if contact.Value == deployer {
			return decoder.unwrapDeployers(contact, events)
		}
		if contact.Value == dbaasServiceChannel {
			return decoder.unwrapDBaaSServiceChannels(contact, events)
		}
		if strings.HasPrefix(contact.Value, groupPrefix) {
			return decoder.unwrapGroup(contact, events)
		}
	}

	defaultResult := []ContactReplacement{{
		ValueReplaced: contact.Value,
		ValueRollback: contact.Value,
	}}
	return defaultResult, nil
}

func (decoder *Decoder) unwrapDuty(contact *moira.ContactData, events moira.NotificationEvents) ([]ContactReplacement, error) {
	items, err := decoder.resolveDuty(contact.Value[len(dutyPrefix):])
	if err != nil {
		return nil, err
	}

	// use only first duty user
	result := []ContactReplacement{{
		Expiration:    items[0].DutyEnd,
		ValueReplaced: "@" + decoder.truncateSuffix(items[0].Login),
		ValueRollback: contact.Value,
	}}
	return result, nil
}

func (decoder *Decoder) unwrapDeployers(contact *moira.ContactData, events moira.NotificationEvents) ([]ContactReplacement, error) {
	const alloc = 32

	duplicates := make(map[string]bool, alloc) // return unique set of deployers
	result := make([]ContactReplacement, 0, alloc)

	for _, event := range events {
		if event.Context == nil {
			continue
		}

		for _, deployerLogin := range event.Context.Deployers {
			deployerLogin = decoder.truncateSuffix(deployerLogin)
			if deployerLogin == "" || duplicates[deployerLogin] {
				continue
			}

			duplicates[deployerLogin] = true
			result = append(result, ContactReplacement{
				Expiration:    nil,
				ValueReplaced: "@" + deployerLogin,
				ValueRollback: "@" + deployerLogin,
			})
		}
	}

	if len(result) == 0 {
		if contact.FallbackValue == "" {
			return nil, ErrNoDeployers{}
		}

		result = append(result, ContactReplacement{
			ValueReplaced: contact.FallbackValue,
			ValueRollback: contact.Value,
		})
	}

	return result, nil
}

func (decoder *Decoder) unwrapDBaaSServiceChannels(contact *moira.ContactData, events moira.NotificationEvents) ([]ContactReplacement, error) {
	const alloc = 32

	duplicates := make(map[string]bool, alloc) // return unique set
	result := make([]ContactReplacement, 0, alloc)

	for _, event := range events {
		if event.Context == nil {
			continue
		}

		for _, channelStruct := range event.Context.ServiceChannels.DBaaS {
			channel := channelStruct.SlackChannel
			if channel == "" {
				continue
			}
			if channel[0] != '#' {
				channel = "#" + channel
			}
			if duplicates[channel] {
				continue
			}

			duplicates[channel] = true
			result = append(result, ContactReplacement{
				Expiration:    nil,
				ValueReplaced: channel,
				ValueRollback: channel,
			})
		}
	}

	if len(result) == 0 {
		if contact.FallbackValue == "" {
			return nil, ErrNoServiceChannels{}
		}

		result = append(result, ContactReplacement{
			ValueReplaced: contact.FallbackValue,
			ValueRollback: contact.Value,
		})
	}

	return result, nil
}

func (decoder *Decoder) unwrapGroup(contact *moira.ContactData, events moira.NotificationEvents) ([]ContactReplacement, error) {
	groupsCache, err := decoder.db.GetSlackUserGroups()
	if err != nil {
		return nil, err
	}

	userGroup := groupsCache[contact.Value[len(groupPrefix):]]
	result := make([]ContactReplacement, 0, len(userGroup.UserIds))
	for _, userId := range userGroup.UserIds {
		result = append(result, ContactReplacement{
			ValueReplaced: userId,
			ValueRollback: userId,
		})
	}

	if len(result) == 0 {
		if contact.FallbackValue == "" {
			return nil, ErrGroupIsEmpty{}
		}

		result = append(result, ContactReplacement{
			ValueReplaced: contact.FallbackValue,
			ValueRollback: contact.Value,
		})
	}

	return result, nil
}

func (decoder *Decoder) truncateSuffix(value string) string {
	if strings.Contains(value, ".ru") {
		value = strings.Split(value, "@")[0]
	}
	return value
}
