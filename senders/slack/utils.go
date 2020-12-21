package slack

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.avito.ru/DO/moira"
)

var AvitoErrbotCommandRegexp *regexp.Regexp

func init() {
	AvitoErrbotCommandRegexp = regexp.MustCompile(
		"(?m)((^_deploy_status_.*)|(^_get_screen.*)|(_include_top_error_)|(_syslog_errors_))\n?",
	)
}

func isCompactOKStyleEnabled(_ *moira.TriggerData) bool {
	return true
}

func isDescriptionInCommentEnabled(_ *moira.TriggerData) bool {
	return true
}

func extractAvitoErrbotCommands(message string) string {
	foundCommands := AvitoErrbotCommandRegexp.FindAllString(message, -1)
	joinedCommands := strings.Join(foundCommands, "")
	joinedCommands = strings.TrimSpace(joinedCommands)
	return joinedCommands
}

func removeAvitoErrbotCommands(message string) string {
	messageWithoutCommands := AvitoErrbotCommandRegexp.ReplaceAllString(message, "")
	messageWithoutCommands = strings.TrimSpace(messageWithoutCommands)
	return messageWithoutCommands
}

// extractMetrics takes NotificationEvents and returns names of all metrics as a sorted []string.
func extractMetrics(events moira.NotificationEvents) []string {
	metrics := make([]string, 0, len(events))
	metricSet := make(map[string]bool, len(events))
	for _, event := range events {
		metricName := event.Metric
		if _, found := metricSet[metricName]; !found {
			metrics = append(metrics, metricName)
			metricSet[metricName] = true
		}
	}
	sort.Strings(metrics)
	return metrics
}

// extractFailedMetrics takes NotificationEvents and returns names of all metrics that failed.
func extractFailedMetrics(events moira.NotificationEvents) []string {
	// duplicate `events` and sort by timestamp
	eventsSorted := append(moira.NotificationEvents(nil), events...)
	sort.Slice(eventsSorted, func(i, j int) bool {
		return eventsSorted[i].Timestamp < eventsSorted[j].Timestamp
	})

	metricState := make(map[string]string, len(eventsSorted))
	for _, event := range eventsSorted {
		metricState[event.Metric] = event.State
	}

	metrics := make([]string, 0, len(metricState))
	for metric, state := range metricState {
		if state != moira.OK {
			metrics = append(metrics, metric)
		}
	}

	return metrics
}

func formatLink(linkText, url string) string {
	if linkText != "" {
		return fmt.Sprintf("<%s|%s>", url, linkText)
	} else {
		return url
	}
}
