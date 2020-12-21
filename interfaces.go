package moira

import (
	"time"

	"gopkg.in/tomb.v2"
)

// Database implements DB functionality
type Database interface {
	// SelfState
	UpdateMetricsHeartbeat() error
	GetMetricsUpdatesCount() (int64, error)
	GetChecksUpdatesCount() (int64, error)

	// Tag storing
	GetTagNames() ([]string, error)
	RemoveTag(tagName string) error
	GetTagTriggerIDs(tagName string) ([]string, error)
	GetTagsStats(tags ...string) ([]TagStats, error)

	// LastCheck storing
	GetTriggerLastCheck(triggerID string) (*CheckData, error)
	GetTriggerLastChecks(triggerIDs []string) (map[string]*CheckData, error)
	GetOrCreateTriggerLastCheck(triggerID string) (*CheckData, error)
	SetTriggerLastCheck(triggerID string, checkData *CheckData) error
	RemoveTriggerLastCheck(triggerID string) error
	GetTriggerCheckIDs(tags []string, onlyErrors bool) ([]string, error)

	// Maintenance operations
	GetMaintenanceSilent(spt SilentPatternType) (Maintenance, error)
	GetMaintenanceTrigger(id string) (Maintenance, error)
	GetOrCreateMaintenanceSilent(spt SilentPatternType) (Maintenance, error)
	GetOrCreateMaintenanceTrigger(id string) (Maintenance, error)
	SetMaintenanceTrigger(id string, maintenance Maintenance) error
	DelMaintenanceTrigger(id string) error

	// Trigger storing
	CheckTriggerExists(string) (bool, error)
	GetTriggerIDs(onlyPull bool) ([]string, error)
	GetTrigger(triggerID string) (*Trigger, error)
	GetTriggers(triggerIDs []string) ([]*Trigger, error)
	GetTriggerChecks(triggerIDs []string) ([]*TriggerCheck, error)
	SaveTrigger(triggerID string, trigger *Trigger) error
	RemoveTrigger(triggerID string) error
	GetPatternTriggerIDs(pattern string) ([]string, error)
	RemovePatternTriggerIDs(pattern string) error
	AddTriggerForcedNotification(triggerID string, metrics []string, time int64) error
	GetTriggerForcedNotifications(triggerID string) (map[string]bool, error)
	DeleteTriggerForcedNotification(triggerID string, metric string) error
	DeleteTriggerForcedNotifications(triggerID string, metrics []string) error

	// Throttling
	GetTriggerThrottling(triggerID string) (time.Time, time.Time)
	SetTriggerThrottling(triggerID string, next time.Time) error
	DeleteTriggerThrottling(triggerID string) error

	// NotificationEvent storing
	GetNotificationEvents(triggerID string, start, size int64) ([]*NotificationEvent, error)
	GetAllNotificationEvents(start, end int64) ([]*NotificationEvent, error)
	PushNotificationEvent(event *NotificationEvent) error
	GetNotificationEventCount(triggerID string, from int64) int64
	FetchNotificationEvent(withSaturations bool) (NotificationEvent, error)
	FetchDelayedNotificationEvents(to int64, withSaturations bool) ([]NotificationEvent, error)
	AddDelayedNotificationEvent(event NotificationEvent, timestamp int64) error

	// Event inheritance
	AddChildEvents(parentTriggerID string, parentMetric string, childTriggerID string, childMetrics []string) error
	GetChildEvents(parentTriggerID, parentMetric string) (map[string][]string, error)
	GetParentEvents(childTriggerID, childMetric string) (map[string][]string, error)
	DeleteChildEvents(parentTriggerID, parentMetric string, childTriggerID string, childMetrics []string) error

	// ContactData storing
	GetContact(contactID string) (ContactData, error)
	GetContacts(contactIDs []string) ([]*ContactData, error)
	GetAllContacts() ([]*ContactData, error)
	RemoveContact(contactID string) error
	SaveContact(contact *ContactData) error
	GetUserContactIDs(userLogin string) ([]string, error)

	// SilentPatterData storing
	GetSilentPatternsAll() ([]*SilentPatternData, error)
	GetSilentPatternsTyped(pt SilentPatternType) ([]*SilentPatternData, error)
	SaveSilentPatterns(pt SilentPatternType, spl ...*SilentPatternData) error
	RemoveSilentPatterns(pt SilentPatternType, spl ...*SilentPatternData) error
	LockSilentPatterns(pt SilentPatternType) error
	UnlockSilentPatterns(pt SilentPatternType) error

	// SubscriptionData storing
	GetSubscription(id string) (SubscriptionData, error)
	GetSubscriptions(subscriptionIDs []string) ([]*SubscriptionData, error)
	GetAllSubscriptions() ([]*SubscriptionData, error)
	MaybeUpdateEscalationsOfSubscription(subscription *SubscriptionData) error
	SaveSubscription(subscription *SubscriptionData) error
	SaveSubscriptions(subscriptions []*SubscriptionData) error
	RemoveSubscription(subscriptionID string) error
	GetUserSubscriptionIDs(userLogin string) ([]string, error)
	GetTagsSubscriptions(tags []string) ([]*SubscriptionData, error)

	// ScheduledNotification storing
	GetNotifications(start, end int64) ([]*ScheduledNotification, int64, error)
	RemoveNotification(notificationKey string) (int64, error)
	FetchNotifications(to int64) ([]*ScheduledNotification, error)
	AddNotification(notification *ScheduledNotification) error
	AddNotifications(notification []*ScheduledNotification, timestamp int64) error

	// Patterns and metrics storing
	GetPatterns() ([]string, error)
	AddPatternMetric(pattern, metric string) error
	GetPatternMetrics(pattern string) ([]string, error)
	RemovePattern(pattern string) error
	RemovePatternsMetrics(pattern []string) error
	RemovePatternWithMetrics(pattern string) error

	SubscribeMetricEvents(tomb *tomb.Tomb) (<-chan *MetricEvent, error)
	SaveMetrics(buffer map[string]*MatchedMetric) error
	GetMetricRetention(metric string) (int64, error)
	GetMetricsValues(metrics []string, from int64, until int64) (map[string][]*MetricValue, error)
	RemoveMetricValues(metric string, toTime int64) error
	RemoveMetricsValues(metrics []string, toTime int64) error

	// Locks
	AcquireLock(lockKey string, ttlSec int, timeout time.Duration) error
	AcquireTriggerCheckLock(triggerID string) error
	AcquireTriggerMaintenanceLock(triggerID string) error
	DeleteLock(lockKey string) error
	DeleteTriggerCheckLock(triggerID string) error
	DeleteTriggerMaintenanceLock(triggerID string) error
	SetLock(lockKey string, ttlSec int) (bool, error)
	SetTriggerCheckLock(triggerID string) (bool, error)
	SetTriggerCoolDown(triggerID string, ttlSec int) (bool, error)

	// Bot data storing
	GetIDByUsername(messenger, username string) (string, error)
	SetUsernameID(messenger, username, id string) error
	RemoveUser(messenger, username string) error
	RegisterBotIfAlreadyNot(messenger string, ttl time.Duration) bool
	RenewBotRegistration(messenger string) bool
	DeregisterBots()
	DeregisterBot(messenger string) bool

	// Escalations
	AddEscalations(ts int64, event NotificationEvent, trigger TriggerData, escalations []EscalationData) error
	TriggerHasPendingEscalations(triggerID string, withResolutions bool) (bool, error)
	MetricHasPendingEscalations(triggerID, metric string, withResolutions bool) (bool, error)
	AckEscalations(triggerID, metric string, withResolutions bool) error
	AckEscalationsBatch(triggerID string, metrics []string, withResolutions bool) error
	FetchScheduledEscalationEvents(to int64) ([]*ScheduledEscalationEvent, error)
	RegisterProcessedEscalationID(escalationID, triggerID, metric string) error
	AddUnacknowledgedMessage(triggerID string, metric string, link MessageLink) error
	GetUnacknowledgedMessages(triggerID, metric string) ([]MessageLink, error)
	AckUnacknowledgedMessages(triggerID, metric string) error

	// Global settings
	GetGlobalSettings() (GlobalSettings, error)
	SetGlobalSettings(GlobalSettings) error

	// Slack-specific
	GetSlackThreadLinks(contactID, triggerID string) (messages map[string]string, err error)
	AddSlackThreadLinks(contactID, triggerID, threadTs, payload string, expiryTime *time.Time) error
	RemoveSlackThreadLinks(contactID, triggerID string, threadsTs, completedThreads []string) error
	GetAllSlackThreadLinks(triggerID string) ([]SlackThreadLink, error)

	GetSlackDashboard(contactID, ts string) (SlackDashboard, error)
	UpdateSlackDashboard(contactID, ts string, db SlackDashboard, expiryTime *time.Time) error
	RemoveSlackDashboards(contactID string, dashboardsTs []string) error
	GetAllInheritedTriggerDashboards(triggerID, ancestorTriggerID, ancestorMetric string) ([]SlackThreadLink, error)
	SaveInheritedTriggerDashboard(contactID, threadTs, triggerID, ancestorTriggerID, ancestorMetric, newDashboardTs string) error
	DeleteInheritedTriggerDashboard(contactID, threadTs, triggerID, ancestorTriggerID, ancestorMetric, dashboardTs string) error

	UpdateInheritanceDataVersion() error

	FetchSlackDelayedActions(until time.Time) ([]SlackDelayedAction, error)
	SaveSlackDelayedAction(action SlackDelayedAction) error
	GetSlackUserGroups() (SlackUserGroupsCache, error)
	SaveSlackUserGroups(userGroups SlackUserGroupsCache) error

	GetServiceDuty(service string) (DutyData, error)
	UpdateServiceDuty(service string, dutyData DutyData) error
}

type TriggerInheritanceDatabase interface {
	Ping() bool

	GetMaxDepthInGraph(id string) (int, error)
	GetAllAncestors(id string) ([][]string, error)
	GetAllChildren(triggerID string) ([]string, error)
	SetTriggerParents(triggerID string, newParentIDs []string) error
}

// Logger implements logger abstraction
type Logger interface {
	Debug(message string)
	DebugE(message string, extra interface{})
	DebugF(format string, args ...interface{})
	Info(message string)
	InfoE(message string, extra interface{})
	InfoF(format string, args ...interface{})
	Warn(message string)
	WarnE(message string, extra interface{})
	WarnF(format string, args ...interface{})
	Error(message string)
	ErrorE(message string, extra interface{})
	ErrorF(format string, args ...interface{})
	Fatal(message string)
	FatalE(message string, extra interface{})
	FatalF(format string, args ...interface{})
	TracePanic(message string, extra interface{})
	TraceSelfStats(id string, started time.Time)
}

// Sender interface for implementing specified contact type sender
type Sender interface {
	Init(senderSettings map[string]string, location *time.Location) error
	SendEvents(events NotificationEvents, contact ContactData, trigger TriggerData, throttled, needAck bool) error
}

// SenderWithInheritance is a sender that can send messages to ancestor triggers' threads
type SenderWithInheritance interface {
	Sender
	SendEventsWithInheritance(
		events NotificationEvents, contact ContactData, trigger TriggerData,
		ancestorTriggerID, ancestorMetric string,
	) error
}

// MessageLink is a link to a message sent in a Sender.
type MessageLink interface {
	// StorageKey is used to serialize a Link to store it in a Database
	StorageKey() string
	FromString(string) error
	SenderName() string
}
