// nolint
package dto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-graphite/carbonapi/pkg/parser"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/middleware"
	"go.avito.ru/DO/moira/expression"
	"go.avito.ru/DO/moira/target"
)

const grafanaPanelID = "panelId"
const grafanaPath = "/grafana/render/dashboard-solo/db/"
const grafanaScriptPath = "/grafana/render/dashboard-solo/script/scripted.js"

var grafanaDisallowedKeys = map[string]struct{}{
	"width":  {},
	"height": {},
}

type TriggersList struct {
	Page  *int64               `json:"page,omitempty"`
	Size  *int64               `json:"size,omitempty"`
	Total *int64               `json:"total,omitempty"`
	List  []moira.TriggerCheck `json:"list"`
}

func (*TriggersList) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type Trigger struct {
	TriggerModel
	Throttling     int64      `json:"throttling"`
	HasEscalations bool       `json:"has_escalations"`
	ParentTriggers []*Trigger `json:"_parent_triggers,omitempty"`
}

// TriggerModel is moira.Trigger api representation
type TriggerModel struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Desc            *string             `json:"desc,omitempty"`
	Targets         []string            `json:"targets"`
	Parents         []string            `json:"parents,omitempty"`
	WarnValue       *float64            `json:"warn_value"`
	ErrorValue      *float64            `json:"error_value"`
	Tags            []string            `json:"tags"`
	TTLState        *string             `json:"ttl_state,omitempty"`
	TTL             int64               `json:"ttl,omitempty"`
	Schedule        *moira.ScheduleData `json:"sched,omitempty"`
	Expression      string              `json:"expression"`
	Patterns        []string            `json:"patterns"`
	IsPullType      bool                `json:"is_pull_type"`
	Dashboard       string              `json:"dashboard"`
	PendingInterval int64               `json:"pending_interval"`
	Maintenance     int64               `json:"maintenance"`
	Saturation      []moira.Saturation  `json:"saturation,omitempty"`
}

// ToMoiraTrigger transforms TriggerModel to moira.Trigger
func (model *TriggerModel) ToMoiraTrigger() *moira.Trigger {
	return &moira.Trigger{
		ID:              model.ID,
		Name:            model.Name,
		Desc:            model.Desc,
		Targets:         model.Targets,
		Parents:         model.Parents,
		WarnValue:       model.WarnValue,
		ErrorValue:      model.ErrorValue,
		Tags:            model.Tags,
		TTLState:        model.TTLState,
		TTL:             model.TTL,
		Schedule:        model.Schedule,
		Expression:      &model.Expression,
		Patterns:        model.Patterns,
		IsPullType:      model.IsPullType,
		Dashboard:       model.Dashboard,
		PendingInterval: model.PendingInterval,
		Saturation:      model.Saturation,
	}
}

// CreateTriggerModel transforms moira.Trigger to TriggerModel
func CreateTriggerModel(trigger *moira.Trigger) TriggerModel {
	return TriggerModel{
		ID:              trigger.ID,
		Name:            trigger.Name,
		Desc:            trigger.Desc,
		Targets:         trigger.Targets,
		Parents:         trigger.Parents,
		WarnValue:       trigger.WarnValue,
		ErrorValue:      trigger.ErrorValue,
		Tags:            trigger.Tags,
		TTLState:        trigger.TTLState,
		TTL:             trigger.TTL,
		Schedule:        trigger.Schedule,
		Expression:      moira.UseString(trigger.Expression),
		Patterns:        trigger.Patterns,
		IsPullType:      trigger.IsPullType,
		Dashboard:       trigger.Dashboard,
		PendingInterval: trigger.PendingInterval,
		Saturation:      trigger.Saturation,
	}
}

func (trigger *Trigger) Bind(request *http.Request) error {
	config := middleware.GetConfig(request)

	if len(trigger.Targets) == 0 {
		return fmt.Errorf("targets is required")
	}
	rewrittenTargets, err := rewriteTargets(trigger.Targets, config.TargetRewriteRules)
	if err != nil {
		return err
	}
	trigger.Targets = rewrittenTargets
	if len(trigger.Tags) == 0 {
		return fmt.Errorf("tags is required")
	}
	reservedTagsFound := checkTriggerTags(trigger.Tags)
	if len(reservedTagsFound) > 0 {
		forbiddenTags := strings.Join(reservedTagsFound, ", ")
		return fmt.Errorf("forbidden tags: %s", forbiddenTags)
	}
	if trigger.Name == "" {
		return fmt.Errorf("trigger name is required")
	}
	if trigger.WarnValue == nil && trigger.Expression == "" {
		return fmt.Errorf("warn_value is required")
	}
	if trigger.ErrorValue == nil && trigger.Expression == "" {
		return fmt.Errorf("error_value is required")
	}
	dashboard, err := resolveDashboard(trigger.Dashboard, config.GrafanaPrefixes)
	if err != nil {
		return err
	}
	trigger.Dashboard = dashboard

	triggerExpression := expression.TriggerExpression{
		AdditionalTargetsValues: make(map[string]float64),
		WarnValue:               trigger.WarnValue,
		ErrorValue:              trigger.ErrorValue,
		PreviousState:           moira.NODATA,
		Expression:              &trigger.Expression,
	}

	if err := resolvePatterns(request, trigger, &triggerExpression); err != nil {
		return err
	}
	if _, err := triggerExpression.Evaluate(); err != nil {
		return err
	}
	return nil
}

func rewriteTargets(targets []string, rewriteRules []api.RewriteRule) ([]string, error) {
	for i, curTarget := range targets {
		parsedExpr, err := target.ParseExpr(curTarget)
		if err != nil {
			return nil, err
		}
		metrics := parsedExpr.Metrics()

		for _, metric := range metrics {
			for _, rule := range rewriteRules {
				if strings.HasPrefix(metric.Metric, rule.From) {
					// making a regexp because the Go stdlib doesn't have a good string searching function
					// this regexp will match `metric.Metric`
					metricRegexp := regexp.MustCompile(regexp.QuoteMeta(metric.Metric))

					matches := metricRegexp.FindAllStringIndex(curTarget, -1)
					// loop over all matches in reverse
					// so that indexes won't change in parts of the `curTarget` that we haven't processed yet
					for i := len(matches) - 1; i >= 0; i-- {
						matchStart := matches[i][0]
						if matchStart > 0 && parser.IsNameChar(curTarget[matchStart-1]) {
							// this match is actually a substring of a longer metric name
							// for example: we're searching for THIS.THING but actually matched not.quite.THIS.THING.yes
							// shouldn't rewrite this match
							continue
						}
						curTarget = curTarget[:matchStart] +
							rule.To +
							strings.TrimPrefix(curTarget[matchStart:], rule.From)
					}

					// no more than one rewrite per metric
					break // for _, rule := range rewriteRules
				}
			}
		}

		targets[i] = curTarget
	}
	return targets, nil
}

func resolveDashboard(dashboard string, prefixes []string) (string, error) {
	if dashboard == "" {
		return "", nil
	}

	if len(prefixes) == 0 {
		return "", fmt.Errorf("grafana is not configured. Dashboard field should be empty")
	}

	// check that dashboard has one of allowed prefixes
	hasRightPrefix := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(dashboard, prefix) {
			hasRightPrefix = true
			break
		}
	}

	if !hasRightPrefix {
		return "", fmt.Errorf("invalid url. should start with any of [%s]", strings.Join(prefixes, ", "))
	}

	parsedURL, err := url.Parse(dashboard)
	if err != nil {
		return "", err
	}
	panelIDValue := parsedURL.Query().Get(grafanaPanelID)
	if panelIDValue == "" {
		return "", fmt.Errorf("grafanaPanelID is not found in url")
	}

	if _, err = strconv.Atoi(panelIDValue); err != nil {
		return "", err
	}

	// rewrite
	return rewriteDashboardURL(*parsedURL)
}

func rewriteDashboardURL(u url.URL) (string, error) {
	v := u.Query()
	for k := range v {
		if _, ok := grafanaDisallowedKeys[k]; ok {
			delete(v, k)
		}
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid path")
	}
	name := parts[len(parts)-1]
	u.Path = grafanaPath + name
	if name == "scripted.js" {
		u.Path = grafanaScriptPath
	}
	u.RawQuery = v.Encode()
	return u.String(), nil
}

func resolvePatterns(request *http.Request, trigger *Trigger, expressionValues *expression.TriggerExpression) error {
	now := time.Now().Unix()
	targetNum := 1
	trigger.Patterns = make([]string, 0)
	timeSeriesNames := make(map[string]bool)

	for _, tar := range trigger.Targets {
		database := middleware.GetDatabase(request)
		result, err := target.EvaluateTarget(database, tar, now-600, now, false)
		if err != nil {
			return err
		}

		trigger.Patterns = append(trigger.Patterns, result.Patterns...)

		if targetNum == 1 {
			expressionValues.MainTargetValue = 42
			for _, timeSeries := range result.TimeSeries {
				timeSeriesNames[timeSeries.Name] = true
			}
		} else {
			targetName := fmt.Sprintf("t%v", targetNum)
			expressionValues.AdditionalTargetsValues[targetName] = 42
		}
		targetNum++
	}
	middleware.SetTimeSeriesNames(request, timeSeriesNames)
	return nil
}

func checkTriggerTags(tags []string) []string {
	reservedTagsFound := make([]string, 0)
	for _, tag := range tags {
		switch tag {
		case moira.EventHighDegradationTag, moira.EventDegradationTag, moira.EventProgressTag:
			reservedTagsFound = append(reservedTagsFound, tag)
		}
	}
	return reservedTagsFound
}

func (*Trigger) Render(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}

type TriggerCheck struct {
	*moira.CheckData
	TriggerID   string            `json:"trigger_id"`
	Maintenance moira.Maintenance `json:"-"`
}

func (triggerCheck *TriggerCheck) Render(_ http.ResponseWriter, _ *http.Request) error {
	if triggerCheck.CheckData == nil {
		return nil
	}

	snapshot := triggerCheck.Maintenance.SnapshotNow()
	for metric, until := range snapshot {
		if state, ok := triggerCheck.Metrics[metric]; ok {
			state.Maintenance = until
			triggerCheck.Metrics[metric] = state
		} else if metric == moira.WildcardMetric {
			triggerCheck.CheckData.Maintenance = until
		}
	}
	return nil
}

type MetricsMaintenance map[string]int64

func (*MetricsMaintenance) Bind(_ *http.Request) error {
	return nil
}

type TriggerMaintenance struct {
	Until int64 `json:"until"`
}

func (*TriggerMaintenance) Bind(_ *http.Request) error {
	return nil
}

type ThrottlingResponse struct {
	Throttling int64 `json:"throttling"`
}

func (*ThrottlingResponse) Render(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}

type SaveTriggerResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (*SaveTriggerResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type TriggerMetrics map[string][]moira.MetricValue

func (*TriggerMetrics) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type AckMetricEscalationsRequest struct {
	Metrics []string `json:"metrics"`
}

func (*AckMetricEscalationsRequest) Bind(request *http.Request) error {
	return nil
}

type UnacknowledgedMessagesRequest struct {
	Metrics []string `json:"metrics"`
}

func (*UnacknowledgedMessagesRequest) Bind(request *http.Request) error {
	return nil
}

type UnacknowledgedMessage struct {
	Sender      string          `json:"sender"`
	MessageLink json.RawMessage `json:"message_link"`
}

type UnacknowledgedMessages []UnacknowledgedMessage

func (*UnacknowledgedMessages) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
