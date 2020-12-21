package moira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Default moira triggers states
const (
	OK        = "OK"
	WARN      = "WARN"
	ERROR     = "ERROR"
	NODATA    = "NODATA"
	EXCEPTION = "EXCEPTION"
	DEL       = "DEL"
	TEST      = "TEST"
)

// Events' mute reasons
const (
	EventMutedNone        = 0
	EventMutedSchedule    = 1
	EventMutedMaintenance = 2
	EventMutedSilent      = 3
)

// fixed offset in minutes and seconds
const (
	FixedTzOffsetMinutes = int64(-180)
	FixedTzOffsetSeconds = FixedTzOffsetMinutes * 60
)

// used as the name of metric to indicate the whole trigger
const (
	WildcardMetric = "*"
)

// limit (in seconds) of trigger's sole check iteration
const (
	TriggerCheckLimit     = 40
	TriggerCheckThreshold = 30
)

const (
	SatTakeScreen SaturationType = "take-screenshot"

	SatGetCMDBDeviceData    SaturationType = "cmdb-device"
	SatCheckPort            SaturationType = "check-port"
	SatGetDBaaSService      SaturationType = "dbaas-storage-owner-service"
	SatGetDBaaSUnit         SaturationType = "dbaas-storage-owner-unit"
	SatGetDeploys           SaturationType = "get-service-deploys"
	SatGetAllDeployStatuses SaturationType = "get-all-deploy-statuses"
	SatRenderDescription    SaturationType = "render-description"
)

var (
	eventStates = [...]string{OK, WARN, ERROR, NODATA, TEST}

	scores = map[string]int64{
		OK:        0,
		DEL:       0,
		WARN:      1,
		ERROR:     100,
		NODATA:    0,
		EXCEPTION: 100000,
	}
)

// NotificationEvent represents trigger state changes event
type NotificationEvent struct {
	ID             string       `json:"id"`
	IsForceSent    bool         `json:"force_sent"`
	IsTriggerEvent bool         `json:"trigger_event"`
	Timestamp      int64        `json:"timestamp"`
	Metric         string       `json:"metric"`
	State          string       `json:"state"`
	OldState       string       `json:"old_state"`
	Value          *float64     `json:"value,omitempty"`
	OldValue       *float64     `json:"old_value,omitempty"`
	TriggerID      string       `json:"trigger_id"`
	SubscriptionID *string      `json:"sub_id,omitempty"`
	ContactID      string       `json:"contactId,omitempty"`
	Message        *string      `json:"msg,omitempty"`
	Batch          *EventsBatch `json:"batch"`

	HasSaturations bool `json:"has_saturations,omitempty"`

	OverriddenByAncestor bool   `json:"overridden,omitempty"`
	DelayedForAncestor   bool   `json:"delayed_for_ancestor,omitempty"`
	AncestorTriggerID    string `json:"ancestor_trigger_id,omitempty"`
	AncestorMetric       string `json:"ancestor_metric,omitempty"`

	// properties related to Fan
	FanTaskID          string                    `json:"fan_task_id,omitempty"`
	WaitingForFanSince int64                     `json:"waiting_for_fan_since"` // this is a timestamp
	Context            *NotificationEventContext `json:"context,omitempty"`
}

// IdempotencyKey is supposed to rule out the possibility of repeated processing NotificationEvent
func (event *NotificationEvent) IdempotencyKey() string {
	var metric string
	if event.IsTriggerEvent {
		metric = WildcardMetric
	} else {
		metric = event.Metric
	}
	return fmt.Sprintf(
		"%s:%d:%s",
		event.TriggerID,
		event.Timestamp,
		metric,
	)
}

// Matches implements gomock.Matcher
func (event *NotificationEvent) Matches(x interface{}) bool {
	other, ok := x.(*NotificationEvent)
	if !ok {
		return false
	}

	val1 := *event
	val1.Batch = nil

	val2 := *other
	val2.Batch = nil

	// 2 events must be equal, except for the Batch
	return reflect.DeepEqual(val1, val2)
}

func (event *NotificationEvent) String() string {
	return fmt.Sprintf(
		"TriggerId: %s, Metric: %s\nValue: %v, OldValue: %v\nState: %s, OldState: %s\nMessage: %s\nTimestamp: %v",
		event.TriggerID, event.Metric,
		UseFloat64(event.Value), UseFloat64(event.OldValue),
		event.State, event.OldState,
		UseString(event.Message),
		event.Timestamp,
	)
}

// EventsBatch is grouping attribute for
// different NotificationEvent`s which were pushed during
// single trigger check
type EventsBatch struct {
	ID   string `json:"id"`
	Time int64  `json:"ts"`
}

func NewEventsBatch(ts int64) *EventsBatch {
	return &EventsBatch{
		ID:   NewStrID(),
		Time: ts,
	}
}

type NotificationEventContext struct {
	Deployers       []string           `json:"deployers,omitempty"`
	Images          []contextImageData `json:"images,omitempty"`
	DeployStatuses  string             `json:"deployStatuses,omitempty"`
	ServiceChannels struct {
		DBaaS []serviceChannel `json:"dbaas,omitempty"`
	} `json:"serviceChannels,omitempty"`
}
type contextImageData struct {
	URL       string `json:"url"`
	SourceURL string `json:"sourceURL"`
	Caption   string `json:"caption,omitempty"`
}
type serviceChannel struct {
	ServiceName  string `json:"serviceName"`
	SlackChannel string `json:"slackChannel"`
}

func (context *NotificationEventContext) UnmarshalJSON(data []byte) error {
	type tmp NotificationEventContext
	if err := json.Unmarshal(data, (*tmp)(context)); err != nil {
		return err
	}
	sort.Strings(context.Deployers)
	return nil
}

func (context *NotificationEventContext) MustMarshal() string {
	if context == nil {
		return ""
	}
	result, err := json.Marshal(context)
	if err != nil {
		panic(err)
	}
	return string(result)
}

// NotificationEvents represents slice of NotificationEvent
type NotificationEvents []NotificationEvent

// GetContext returns the context of the events
// we assume that all events have the same context
func (events NotificationEvents) GetContext() *NotificationEventContext {
	if len(events) == 0 {
		return nil
	}
	return events[0].Context
}

// GetSubjectState returns the most critical state of events
func (events NotificationEvents) GetSubjectState() string {
	result := ""
	states := make(map[string]bool)
	for _, event := range events {
		states[event.State] = true
	}
	for _, state := range eventStates {
		if states[state] {
			result = state
		}
	}
	return result
}

// TriggerData represents trigger object
type TriggerData struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Desc       string       `json:"desc"`
	Targets    []string     `json:"targets"`
	Parents    []string     `json:"parents"`
	WarnValue  float64      `json:"warn_value"`
	ErrorValue float64      `json:"error_value"`
	Tags       []string     `json:"__notifier_trigger_tags"`
	Dashboard  string       `json:"dashboard"`
	Saturation []Saturation `json:"saturation"`
}

// GetTags returns "[tag1][tag2]...[tagN]" string
func (trigger *TriggerData) GetTags() string {
	var buffer bytes.Buffer
	for _, tag := range trigger.Tags {
		buffer.WriteString(fmt.Sprintf("[%s]", tag))
	}
	return buffer.String()
}

type Saturation struct {
	Type            SaturationType  `json:"type"`
	Fallback        string          `json:"fallback,omitempty"`
	ExtraParameters json.RawMessage `json:"extra_parameters,omitempty"`
}

type SaturationType string

// ContactData represents contact object
type ContactData struct {
	Type          string `json:"type"`
	Value         string `json:"value"`
	FallbackValue string `json:"fallback_value,omitempty"`
	ID            string `json:"id"`
	User          string `json:"user"` // User is the user that _created_ the contact
	Expiration    *time.Time
}

func (cd *ContactData) NeedsFallbackValue() bool {
	return cd.Type == "slack" && cd.Value[0] == '_'
}

type SilentPatternData struct {
	ID      string            `json:"id"`
	Login   string            `json:"login"`
	Pattern string            `json:"pattern"`
	Created int64             `json:"created_at"`
	Until   int64             `json:"until"`
	Type    SilentPatternType `json:"type"`
}

func (spd *SilentPatternData) IsMetric() bool {
	return spd.Type == SPTMetric
}

func (spd *SilentPatternData) IsTag() bool {
	return spd.Type == SPTTag
}

type SilentPatternType int

const (
	SPTMetric SilentPatternType = 0
	SPTTag    SilentPatternType = 1
)

// EscalationData represents escalation object
type EscalationData struct {
	ID              string   `json:"id"`
	Contacts        []string `json:"contacts"`
	OffsetInMinutes int64    `json:"offset_in_minutes"`
}

// SubscriptionData represent user subscription
type SubscriptionData struct {
	Contacts          []string         `json:"contacts"`
	Tags              []string         `json:"tags"`
	Schedule          ScheduleData     `json:"sched"`
	ID                string           `json:"id"`
	Enabled           bool             `json:"enabled"`
	ThrottlingEnabled bool             `json:"throttling"`
	User              string           `json:"user"`
	Escalations       []EscalationData `json:"escalations"`
}

// ScheduleData represent subscription schedule
type ScheduleData struct {
	Days           []ScheduleDataDay `json:"days"`
	TimezoneOffset int64             `json:"tzOffset"`
	StartOffset    int64             `json:"startOffset"`
	EndOffset      int64             `json:"endOffset"`
}

// GetFixedTzOffset returns Moscow tz offset in minutes
func (schedule *ScheduleData) GetFixedTzOffset() int64 {
	return int64(-180)
}

// IsScheduleAllows check if the time is in the allowed schedule interval
func (schedule *ScheduleData) IsScheduleAllows(eventTs int64) bool {
	if schedule == nil {
		return true
	}

	eventTs = eventTs - eventTs%60 - FixedTzOffsetSeconds // truncate to minutes
	eventTime := time.Unix(eventTs, 0).UTC()
	eventWeekday := eventTime.Weekday()

	// points converted to seconds relative to the day
	eventTs = eventTs % 86400
	scheduleStart := schedule.StartOffset * 60
	scheduleEnd := schedule.EndOffset * 60

	if scheduleStart > scheduleEnd { // "inverted" schedule, e.g. 22:00 - 08:00
		// there are 2 possible ways of moments' disposition:
		// 1) schedule start -> event -> midnight -> schedule end
		// 2) schedule start -> midnight -> event -> schedule end
		isEventPastMidnight := eventTs < scheduleEnd
		// if event happened after midnight (the 2nd case) then the previous day enable flag is taken
		if !schedule.isScheduleDaysAllows(eventWeekday, isEventPastMidnight) {
			return false
		}

		return (scheduleStart <= eventTs && !isEventPastMidnight) || (eventTs <= scheduleEnd && isEventPastMidnight)
	} else { // "regular" schedule, e.g. 09:00 - 18:00
		if !schedule.isScheduleDaysAllows(eventWeekday, false) {
			return false
		}

		return scheduleStart <= eventTs && eventTs <= scheduleEnd
	}
}

// isScheduleDaysAllows can tell if the particular day of the week is enabled by the schedule
// dayBefore indicates that the day before must be considered instead of the given day
func (schedule *ScheduleData) isScheduleDaysAllows(weekday time.Weekday, dayBefore bool) bool {
	var (
		daysOffset int
	)

	if dayBefore {
		daysOffset = 1
	} else {
		daysOffset = 0
	}

	return schedule.Days[(int(weekday+6)-daysOffset)%7].Enabled
}

// ScheduleDataDay represent week day of schedule
type ScheduleDataDay struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name,omitempty"`
}

// ScheduledNotification represent notification object
type ScheduledNotification struct {
	Event     NotificationEvent `json:"event"`
	Trigger   TriggerData       `json:"trigger"`
	Contact   ContactData       `json:"contact"`
	Timestamp int64             `json:"timestamp"`
	SendFail  int               `json:"send_fail"`
	NeedAck   bool              `json:"need_ack"`
	Throttled bool              `json:"throttled"`
}

// GetKey return notification key to prevent duplication to the same contact
func (notification *ScheduledNotification) GetKey() string {
	var prefix string
	if notification.Event.AncestorTriggerID == "" {
		prefix = fmt.Sprintf(
			"%s:%s",
			notification.Contact.Type,
			notification.Contact.Value,
		)
	} else {
		// if the notification event has an ancestor, we ignore the contact name
		prefix = fmt.Sprintf(
			"%s",
			notification.Contact.Type,
		)
	}
	return fmt.Sprintf("%s:%s:%s:%s:%d:%f:%d:%d",
		prefix,
		notification.Event.TriggerID,
		notification.Event.Metric,
		notification.Event.State,
		notification.Event.Timestamp,
		UseFloat64(notification.Event.Value),
		notification.SendFail,
		notification.Timestamp,
	)
}

// TagStats wraps trigger ids and subscriptions which are related to the given tag
type TagStats struct {
	Name          string             `json:"name"`
	Triggers      []string           `json:"triggers"`
	Subscriptions []SubscriptionData `json:"subscriptions"`
}

// MatchedMetric represent parsed and matched metric data
type MatchedMetric struct {
	Metric             string
	Patterns           []string
	Value              float64
	Timestamp          int64
	RetentionTimestamp int64
	Retention          int
}

// MetricValue represent metric data
type MetricValue struct {
	RetentionTimestamp int64   `json:"step,omitempty"`
	Timestamp          int64   `json:"ts"`
	Value              float64 `json:"value"`
}

// Trigger represents trigger data object
type Trigger struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Desc             *string       `json:"desc,omitempty"`
	Targets          []string      `json:"targets"`
	Parents          []string      `json:"parents"`
	WarnValue        *float64      `json:"warn_value"`
	ErrorValue       *float64      `json:"error_value"`
	Tags             []string      `json:"tags"`
	TTLState         *string       `json:"ttl_state,omitempty"`
	TTL              int64         `json:"ttl,omitempty"`
	Schedule         *ScheduleData `json:"sched,omitempty"`
	Expression       *string       `json:"expression,omitempty"`
	PythonExpression *string       `json:"python_expression,omitempty"`
	Patterns         []string      `json:"patterns"`
	IsPullType       bool          `json:"is_pull_type"`
	Dashboard        string        `json:"dashboard"`
	PendingInterval  int64         `json:"pending_interval"`
	Saturation       []Saturation  `json:"saturation"`
}

// IsSimple checks triggers patterns
// If patterns more than one or it contains standard graphite wildcard symbols,
// when this target can contain more then one metrics, and is it not simple trigger
func (trigger *Trigger) IsSimple() bool {
	if len(trigger.Targets) > 1 || len(trigger.Patterns) > 1 {
		return false
	}
	for _, pattern := range trigger.Patterns {
		if strings.ContainsAny(pattern, "*{?[") {
			return false
		}
	}
	return true
}

// TriggerCheck represent trigger data with last check data and check timestamp
type TriggerCheck struct {
	Trigger
	Throttling int64      `json:"throttling"`
	LastCheck  *CheckData `json:"last_check"`
}

// CheckData represent last trigger check data
type CheckData struct {
	IsPending         bool                    `json:"is_pending"`
	Message           string                  `json:"msg,omitempty"`
	Timestamp         int64                   `json:"timestamp,omitempty"`
	EventTimestamp    int64                   `json:"event_timestamp,omitempty"`
	Score             int64                   `json:"score"`
	State             string                  `json:"state"`
	Suppressed        bool                    `json:"suppressed,omitempty"`
	Maintenance       int64                   `json:"maintenance,omitempty"`
	MaintenanceMetric map[string]int64        `json:"maintenance_metric,omitempty"`
	Metrics           map[string]*MetricState `json:"metrics"`
	Version           int                     `json:"version"`
}

// GetEventTimestamp gets event timestamp for given check
func (checkData CheckData) GetEventTimestamp() int64 {
	if checkData.EventTimestamp == 0 {
		return checkData.Timestamp
	}
	return checkData.EventTimestamp
}

// GetOrCreateMetricState gets metric state from check data or create new if CheckData has no state for given metric
func (checkData *CheckData) GetOrCreateMetricState(metric string, emptyTimestampValue int64) *MetricState {
	_, ok := checkData.Metrics[metric]
	if !ok {
		checkData.Metrics[metric] = &MetricState{
			IsNoData:  true,
			State:     NODATA,
			Timestamp: emptyTimestampValue,
		}
	}
	return checkData.Metrics[metric]
}

// UpdateScore update and return checkData score, based on metric states and checkData state
func (checkData *CheckData) UpdateScore() int64 {
	checkData.Score = scores[checkData.State]
	for _, metricData := range checkData.Metrics {
		checkData.Score += scores[metricData.State]
	}
	return checkData.Score
}

// MetricState represent metric state data for given timestamp
type MetricState struct {
	EventTimestamp int64    `json:"event_timestamp"`
	State          string   `json:"state"`
	Suppressed     bool     `json:"suppressed"`
	Timestamp      int64    `json:"timestamp"`
	Value          *float64 `json:"value,omitempty"`
	Maintenance    int64    `json:"maintenance,omitempty"`
	IsPending      bool     `json:"is_pending"`

	IsNoData bool `json:"is_no_data"`
	IsForced bool `json:"is_forced,omitempty"`
}

// GetCheckPoint gets check point for given MetricState
// CheckPoint is the timestamp from which to start checking the current state of the metric
func (metricState *MetricState) GetCheckPoint(checkPointGap int64) int64 {
	return MaxI64(metricState.Timestamp-checkPointGap, metricState.EventTimestamp)
}

// GetEventTimestamp gets event timestamp for given metric
func (metricState *MetricState) GetEventTimestamp() int64 {
	if metricState.EventTimestamp == 0 {
		return metricState.Timestamp
	}
	return metricState.EventTimestamp
}

// MetricEvent represent filter metric event
type MetricEvent struct {
	Metric  string `json:"metric"`
	Pattern string `json:"pattern"`
}

// maintenanceInterval is maintenance interval for some metric or for the whole trigger
type maintenanceInterval struct {
	From  int64 `json:"from"`
	Until int64 `json:"until"`
}

// Maintenance is history of maintenance intervals for each metric of the trigger
// key for the whole trigger maintenance is WildcardMetric
type Maintenance map[string][]maintenanceInterval

// NewMaintenance creates blank Maintenance instance
func NewMaintenance() Maintenance {
	return make(Maintenance)
}

// NewMaintenanceFromCheckData migrates CheckData maintenance to the new Maintenance instance
// only maintenanceInterval.Until values can be filled
func NewMaintenanceFromCheckData(data *CheckData) Maintenance {
	result := make(Maintenance, len(data.MaintenanceMetric)+1)
	if data.Maintenance > 0 {
		result[WildcardMetric] = []maintenanceInterval{{Until: data.Maintenance}}
	}
	for metric, maintenance := range data.MaintenanceMetric {
		result[metric] = []maintenanceInterval{{Until: maintenance}}
	}
	return result
}

// Add adds maintenance for the given metric
func (maintenance Maintenance) Add(metric string, until int64) {
	now := time.Now().Unix()
	history, ok := maintenance[metric]

	if ok {
		last := &history[len(history)-1]
		if last.Until > now {
			// last maintenance isn't over yet, extend it
			last.Until = until
		} else {
			// append new maintenance
			history = append(history, maintenanceInterval{
				From:  now,
				Until: until,
			})
			maintenance[metric] = history
		}
	} else {
		maintenance[metric] = []maintenanceInterval{{
			From:  now,
			Until: until,
		}}
	}
}

// Get returns maintenance state for the given metric at the given timestamp
func (maintenance Maintenance) Get(metric string, ts int64) (maintained bool, until int64) {
	// check the metric is present in the first place
	history, ok := maintenance[metric]
	if !ok {
		return false, 0
	}
	qty := len(history)

	// look for the interval where Until exceeds ts
	pos := sort.Search(qty, func(i int) bool {
		return history[i].Until >= ts
	})
	if pos == qty { // sort.Search returns collection's length if nothing has been found
		return false, 0
	}
	return true, history[pos].Until
}

// Del terminates maintenance for the given metric
// it does nothing if the metric doesn't exist
func (maintenance Maintenance) Del(metric string) {
	if history, ok := maintenance[metric]; ok {
		history[len(history)-1].Until = time.Now().Unix()
		maintenance[metric] = history
	}
}

// Clean removes all outdated maintenance intervals
func (maintenance Maintenance) Clean() {
	const housekeepingRange = 30 * 24 * 60 * 60 // 30 days
	margin := time.Now().Unix() - housekeepingRange

	for metric, history := range maintenance {
		// find first non-outdated interval
		qty := len(history)
		pos := sort.Search(qty, func(i int) bool {
			return history[i].From > margin
		})

		if pos == qty {
			// all intervals are outdated -- just delete metric entry
			delete(maintenance, metric)
		} else if pos > 0 {
			// truncate intervals
			maintenance[metric] = history[pos:]
		}
	}
}

// Snapshot returns map of metrics to their actual maintenance on the given timestamp
func (maintenance Maintenance) Snapshot(ts int64) map[string]int64 {
	result := make(map[string]int64, len(maintenance))
	for metric, history := range maintenance {
		qty := len(history)
		pos := sort.Search(qty, func(i int) bool {
			return history[i].Until >= ts
		})

		if pos < qty {
			result[metric] = history[pos].Until
		}
	}
	return result
}

func (maintenance Maintenance) SnapshotNow() map[string]int64 {
	return maintenance.Snapshot(time.Now().Unix())
}

// ScheduledEscalationEvent represent escalated notification event
type ScheduledEscalationEvent struct {
	Escalation   EscalationData    `json:"escalation"`
	Event        NotificationEvent `json:"event"`
	Trigger      TriggerData       `json:"trigger"`
	IsFinal      bool              `json:"is_final"`
	IsResolution bool              `json:"is_resolution"`
}

type NotificationsDisabledSettings struct {
	Author   string `json:"author"`
	Disabled bool   `json:"disabled"`
}

type GlobalSettings struct {
	Notifications NotificationsDisabledSettings `json:"notifications"`
}

type DutyItem struct {
	Login   string
	DutyEnd *time.Time `json:"duty_end"`
}

type DutyData struct {
	Duty      []DutyItem
	Timestamp time.Time
}

type RateLimit struct {
	AcceptRate float64
	ThreadsQty int
}

type SlackDelayedAction struct {
	Action      string          `json:"action"`
	EncodedArgs json.RawMessage `json:"encodedArgs"`
	FailCount   int             `json:"failCount"`
	ScheduledAt time.Time       `json:"scheduledAt"`

	// these fields are only used for logging
	Contact ContactData `json:"contact"`
}

// SlackUserGroup represents slack user group macro (user to mention several users at a time)
type SlackUserGroup struct {
	Id         string    `json:"id"`
	Handle     string    `json:"handle"` // macro
	Name       string    `json:"name"`   // human-readable name
	DateCreate time.Time `json:"date_create"`
	DateUpdate time.Time `json:"date_update"`
	UserIds    []string  `json:"user_ids"` // sadly, not names, but slack accepts it
}

// SlackUserGroupsCache maps SlackUserGroup.Handle to SlackUserGroup
type SlackUserGroupsCache map[string]SlackUserGroup
