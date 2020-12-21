package slack

import (
	"fmt"
	"sort"
	"strings"

	"go.avito.ru/DO/moira"
)

func RenderDashboardForSlack(sb moira.SlackDashboard, maxLines int) string {
	const (
		okEmoji  = ":jr-approve:"
		badEmoji = ":x:"
	)

	// sort the metrics before outputting
	sortedMetrics := make([]string, len(sb))
	i := 0
	for metric, _ := range sb {
		sortedMetrics[i] = metric
		i++
	}
	sort.Strings(sortedMetrics)
	sort.SliceStable(sortedMetrics, func(i, j int) bool {
		// if sortedMetrics[i] is in error...
		if !sb[sortedMetrics[i]] {
			// if sortedMetrics[j] is NOT in error...
			// ...then show sortedMetrics[i] before sortedMetrics[j]
			if sb[sortedMetrics[j]] {
				return true
			}
		}
		return false
	})

	result := strings.Builder{}

	// output the first `maxLines` metrics
	var numOfMetricsToOutput int
	if maxLines == 0 {
		numOfMetricsToOutput = len(sortedMetrics)
	} else {
		numOfMetricsToOutput = min(maxLines, len(sortedMetrics))
	}
	for i := 0; i < numOfMetricsToOutput; i++ {
		metric := sortedMetrics[i]
		if sb[metric] {
			result.WriteString(okEmoji)
		} else {
			result.WriteString(badEmoji)
		}
		result.WriteString(" " + metric + "\n")
	}

	if numOfMetricsToOutput != len(sortedMetrics) {
		// there are more metrics than `maxLines`
		// output the rest of them compressed

		var total, totalOK, totalBad int
		for j := numOfMetricsToOutput; j < len(sortedMetrics); j++ {
			metric := sortedMetrics[j]
			if sb[metric] {
				totalOK += 1
			} else {
				totalBad += 1
			}
			total += 1
		}

		result.WriteString(fmt.Sprintf(
			"(and %d more: %d %s, %d %s)\n",
			total, totalBad, badEmoji, totalOK, okEmoji,
		))
	}

	return result.String()
}

func min(a, b int) int {
	// `min` is apparently too complicated to be included in Golang stdlib
	// so here we are
	if a < b {
		return a
	} else {
		return b
	}
}
