package moira

import (
	"fmt"
	"strings"
)

type SlackDashboard map[string]bool // map from metric to isOK

func (sb *SlackDashboard) Update(events NotificationEvents, triggerName string) (hasChanged bool, changes NotificationEvents) {
	hasChanged = false
	changes = make(NotificationEvents, 0, len(events))
	for _, event := range events {
		if event.State == OK {
			if oldState, ok := (*sb)[event.Metric]; ok && !oldState {
				hasChanged = true
				(*sb)[event.Metric] = true
				changes = append(changes, event)
			}
		}
		if !event.IsTriggerEvent {
			if oldState, ok := (*sb)[triggerName]; ok && !oldState {
				hasChanged = true
				(*sb)[triggerName] = true
				changes = append(changes, event)
			}
		}
	}
	return hasChanged, changes
}

func (sb SlackDashboard) IsEverythingOK() bool {
	for _, isOK := range sb {
		if !isOK {
			return false
		}
	}
	return true
}

func MakeDashboardFromEvents(events NotificationEvents) SlackDashboard {
	result := make(SlackDashboard)
	for _, event := range events {
		result[event.Metric] = event.State == OK
	}
	return result
}

type SlackThreadLink struct {
	Contact     string `json:"contact"`
	ThreadTs    string `json:"thread_ts"`
	DashboardTs string `json:"dashboard_ts,omitempty"`
}

func (link SlackThreadLink) StorageKey() string {
	if link.DashboardTs != "" {
		return fmt.Sprintf("slack:%s:%s:%s", link.Contact, link.ThreadTs, link.DashboardTs)
	} else {
		return fmt.Sprintf("slack:%s:%s", link.Contact, link.ThreadTs)
	}
}

func (link *SlackThreadLink) FromString(s string) error {
	split := strings.SplitN(s, ":", 3)
	if len(split) < 2 {
		return fmt.Errorf("failed to parse %s as SlackThreadLink", s)
	}
	link.Contact = split[0]
	link.ThreadTs = split[1]
	if len(split) >= 3 {
		link.DashboardTs = split[2]
	}
	return nil
}

func (link SlackThreadLink) SenderName() string {
	return "slack"
}
