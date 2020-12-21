package target

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/go-graphite/carbonapi/expr"
	"github.com/go-graphite/carbonapi/expr/functions"
	"github.com/go-graphite/carbonapi/expr/rewrite"
	"github.com/go-graphite/carbonapi/expr/types"
	"github.com/go-graphite/carbonapi/pkg/parser"

	"go.avito.ru/DO/moira"
)

// EvaluationResult represents evaluation target result and contains TimeSeries list, Pattern list and metric lists appropriate to given target
type EvaluationResult struct {
	TimeSeries []*TimeSeries
	Patterns   []string
	Metrics    []string
}

// ParseExpr calls parser.ParseExpr and wraps its error.
func ParseExpr(target string) (parser.Expr, error) {
	// target may contain unexpected quotes, especially when sent through api
	target = strings.Trim(target, "'\"")
	parsed, _, err := parser.ParseExpr(target)
	if err != nil {
		return nil, ErrParseExpr{
			internalError: err,
			target:        target,
		}
	}
	return parsed, err
}

// EvaluateTarget is analogue of evaluateTarget method in graphite-web, that gets target metrics value from DB and Evaluate it using carbon-api eval package
func EvaluateTarget(database moira.Database, target string, from int64, until int64, allowRealTimeAlerting bool) (*EvaluationResult, error) {
	result := &EvaluationResult{
		TimeSeries: make([]*TimeSeries, 0),
		Patterns:   make([]string, 0),
		Metrics:    make([]string, 0),
	}

	targets := []string{target}
	targetIdx := 0
	for targetIdx < len(targets) {
		target := targets[targetIdx]
		targetIdx++

		parsed, err := ParseExpr(target)
		if err != nil {
			return nil, err
		}

		patterns := parsed.Metrics()
		metricsMap, metrics, err := getPatternsMetricData(database, patterns, from, until, allowRealTimeAlerting)
		if err != nil {
			return nil, err
		}

		rewritten, newTargets, err := expr.RewriteExpr(parsed, int32(from), int32(until), metricsMap)
		if err != nil && err != parser.ErrSeriesDoesNotExist {
			return nil, fmt.Errorf("Failed RewriteExpr: %s", err.Error())
		}
		if rewritten {
			targets = append(targets, newTargets...)
			continue
		}

		metricsData, err := func() (result []*types.MetricData, err error) {
			defer func() {
				if r := recover(); r != nil {
					result = nil
					err = fmt.Errorf("panic while evaluate target %s: message: '%s' stack: %s", target, r, debug.Stack())
				}
			}()

			result, err = expr.EvalExpr(parsed, int32(from), int32(until), metricsMap)
			if err != nil {
				if err == parser.ErrSeriesDoesNotExist {
					err = nil
				} else {
					err = ErrEvalExpr{
						target:        target,
						internalError: err,
					}
				}
			}
			return result, err
		}()
		if err != nil {
			return nil, err
		}

		for _, metricData := range metricsData {
			timeSeries := TimeSeries{
				MetricData: *metricData,
				Wildcard:   len(metrics) == 0,
			}
			result.TimeSeries = append(result.TimeSeries, &timeSeries)
		}
		result.Metrics = append(result.Metrics, metrics...)
		for _, pattern := range patterns {
			result.Patterns = append(result.Patterns, pattern.Metric)
		}
	}
	return result, nil
}

func getPatternsMetricData(database moira.Database, patterns []parser.MetricRequest, from int64, until int64, allowRealTimeAlerting bool) (map[parser.MetricRequest][]*types.MetricData, []string, error) {
	metrics := make([]string, 0)
	metricsMap := make(map[parser.MetricRequest][]*types.MetricData)
	for _, pattern := range patterns {
		pattern.From += int32(from)
		pattern.Until += int32(until)
		metricDatas, patternMetrics, err := FetchData(database, pattern.Metric, int64(pattern.From), int64(pattern.Until), allowRealTimeAlerting)
		if err != nil {
			return nil, nil, err
		}
		metricsMap[pattern] = metricDatas
		metrics = append(metrics, patternMetrics...)
	}
	return metricsMap, metrics, nil
}

func init() {
	// need to call both functions.New and rewrite.New explicitly so that carbonapi could register its functions (and rewrites)
	functions.New(make(map[string]string))
	rewrite.New(make(map[string]string))
}
