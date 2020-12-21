package filter

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/mock/moira-alert"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestParseMetricFromString(t *testing.T) {
	type ValidMetricCase struct {
		raw       string
		metric    string
		value     float64
		timestamp int64
	}

	Convey("Given invalid metric strings, should return errors", t, func() {
		invalidMetrics := []string{
			"Invalid.value 12g5 1234567890",
			"No.value.two.spaces  1234567890",
			"No.timestamp.space.in.the.end 12 ",
			"No.timestamp 12",
			" 12 1234567890",
			"Non-ascii.こんにちは 12 1234567890",
			"Non-printable.\000 12 1234567890",
			"",
			"\n",
			"Too.many.parts 1 2 3 4 12 1234567890",
			"Space.in.the.end 12 1234567890 ",
			" Space.in.the.beginning 12 1234567890",
			"\tNon-printable.in.the.beginning 12 1234567890",
			"\rNon-printable.in.the.beginning 12 1234567890",
			"Newline.in.the.end 12 1234567890\n",
			"Newline.in.the.end 12 1234567890\r",
			"Newline.in.the.end 12 1234567890\r\n",
		}

		for _, invalidMetric := range invalidMetrics {
			_, _, _, err := parseMetricFromString([]byte(invalidMetric))
			So(err, ShouldBeError)
		}
	})

	Convey("Given valid metric strings, should return parsed values", t, func() {
		validMetrics := []ValidMetricCase{
			{"One.two.three 123 1234567890", "One.two.three", 123, 1234567890},
			{"One.two.three 1.23e2 1234567890", "One.two.three", 123, 1234567890},
			{"One.two.three -123 1234567890", "One.two.three", -123, 1234567890},
			{"One.two.three +123 1234567890", "One.two.three", 123, 1234567890},
			{"One.two.three 123. 1234567890", "One.two.three", 123, 1234567890},
			{"One.two.three 123.0 1234567890", "One.two.three", 123, 1234567890},
			{"One.two.three .123 1234567890", "One.two.three", 0.123, 1234567890},
		}

		for _, validMetric := range validMetrics {
			metric, value, timestamp, err := parseMetricFromString([]byte(validMetric.raw))
			So(err, ShouldBeEmpty)
			So(metric, ShouldResemble, validMetric.metric)
			So(value, ShouldResemble, validMetric.value)
			So(timestamp, ShouldResemble, validMetric.timestamp)
		}
	})

	Convey("Given valid metric strings with float64 timestamp, should return parsed values", t, func() {
		var testTimestamp int64 = 1234567890

		// Create and test n metrics with float64 timestamp with fractional part of length n (n=19)
		//
		// For example:
		//
		// [n=1] One.two.three 123 1234567890.6
		// [n=2] One.two.three 123 1234567890.94
		// [n=3] One.two.three 123 1234567890.665
		// [n=4] One.two.three 123 1234567890.4377
		// ...
		// [n=19] One.two.three 123 1234567890.6790847778320312500

		for i := 1; i < 20; i++ {
			rawTimestamp := strconv.FormatFloat(float64(testTimestamp)+rand.Float64(), 'f', i, 64)
			rawMetric := "One.two.three 123 " + rawTimestamp
			validMetric := ValidMetricCase{rawMetric, "One.two.three", 123, testTimestamp}
			metric, value, timestamp, err := parseMetricFromString([]byte(validMetric.raw))
			So(err, ShouldBeEmpty)
			So(metric, ShouldResemble, validMetric.metric)
			So(value, ShouldResemble, validMetric.value)
			So(timestamp, ShouldResemble, validMetric.timestamp)
		}
	})
}

func TestProcessIncomingMetric(t *testing.T) {
	testPatterns := []string{
		"Simple.matching.pattern",
		"Star.single.*",
		"Star.*.double.any*",
		"Bracket.{one,two,three}.pattern",
		"Bracket.pr{one,two,three}suf",
		"Complex.matching.pattern",
		"Complex.*.*",
		"Complex.*.",
		"Complex.*{one,two,three}suf*.pattern",
		"Question.?at_begin",
		"Question.at_the_end?",
	}

	nonMatchingMetrics := []string{
		"Simple.notmatching.pattern",
		"Star.nothing",
		"Bracket.one.nothing",
		"Bracket.nothing.pattern",
		"Complex.prefixonesuffix",
	}

	matchingMetrics := []string{
		"Simple.matching.pattern",
		"Star.single.anything",
		"Star.anything.double.anything",
		"Bracket.one.pattern",
		"Bracket.two.pattern",
		"Bracket.three.pattern",
		"Bracket.pronesuf",
		"Bracket.prtwosuf",
		"Bracket.prthreesuf",
		"Complex.matching.pattern",
		"Complex.anything.pattern",
		"Complex.prefixonesuffix.pattern",
		"Complex.prefixtwofix.pattern",
		"Complex.anything.pattern",
		"Question.1at_begin",
		"Question.at_the_end2",
	}

	mockCtrl := gomock.NewController(t)
	database := mock_moira_alert.NewMockDatabase(mockCtrl)

	test_helpers.InitTestLogging()
	logger := test_helpers.GetTestLogger()

	Convey("Create new pattern storage, GetPatterns returns error, should error", t, func() {
		database.EXPECT().GetPatterns().Return(nil, fmt.Errorf("Some error here"))
		_, err := NewPatternStorage(database, metrics.NewFilterMetrics(), logger)
		So(err, ShouldBeError, fmt.Errorf("Some error here"))
	})

	database.EXPECT().GetPatterns().Return(testPatterns, nil)
	patternsStorage, err := NewPatternStorage(database, metrics.NewFilterMetrics(), logger)

	Convey("Create new pattern storage, should no error", t, func() {
		So(err, ShouldBeEmpty)
	})

	Convey("When invalid metric arrives, should be properly counted", t, func() {
		matchedMetrics := patternsStorage.ProcessIncomingMetric([]byte("Invalid.metric"))
		So(matchedMetrics, ShouldBeNil)
		So(patternsStorage.metrics.TotalMetricsReceived.GetCount(), ShouldEqual, 1)
		So(patternsStorage.metrics.ValidMetricsReceived.GetCount(), ShouldEqual, 0)
		So(patternsStorage.metrics.MatchingMetricsReceived.GetCount(), ShouldEqual, 0)
	})

	Convey("When valid non-matching metric arrives", t, func() {
		Convey("When metric arrives with int64 timestamp", func() {
			patternsStorage.metrics = metrics.NewFilterMetrics()
			patternsStorage.metrics.TotalMetricsReceived.Flush()

			for _, metric := range nonMatchingMetrics {
				matchedMetrics := patternsStorage.ProcessIncomingMetric([]byte(metric + " 12 1234567890"))
				So(matchedMetrics, ShouldBeNil)
			}
			So(patternsStorage.metrics.TotalMetricsReceived.GetCount(), ShouldEqual, len(nonMatchingMetrics))
			So(patternsStorage.metrics.ValidMetricsReceived.GetCount(), ShouldEqual, len(nonMatchingMetrics))
			So(patternsStorage.metrics.MatchingMetricsReceived.GetCount(), ShouldEqual, 0)
		})

		Convey("When metric arrives with float64 timestamp", func() {
			patternsStorage.metrics = metrics.NewFilterMetrics()
			patternsStorage.metrics.TotalMetricsReceived.Flush()

			for _, metric := range nonMatchingMetrics {
				matchedMetrics := patternsStorage.ProcessIncomingMetric([]byte(metric + " 12 1234567890.0"))
				So(matchedMetrics, ShouldBeNil)
			}
			So(patternsStorage.metrics.TotalMetricsReceived.GetCount(), ShouldEqual, len(nonMatchingMetrics))
			So(patternsStorage.metrics.ValidMetricsReceived.GetCount(), ShouldEqual, len(nonMatchingMetrics))
			So(patternsStorage.metrics.MatchingMetricsReceived.GetCount(), ShouldEqual, 0)
		})
	})

	Convey("When valid matching metric arrives", t, func() {
		Convey("When metric name is pure", func() {
			patternsStorage.metrics = metrics.NewFilterMetrics()
			patternsStorage.metrics.TotalMetricsReceived.Flush()

			for _, metric := range matchingMetrics {
				matchedMetrics := patternsStorage.ProcessIncomingMetric([]byte(metric + " 12 1234567890"))
				So(matchedMetrics, ShouldNotBeNil)
			}
			So(patternsStorage.metrics.TotalMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
			So(patternsStorage.metrics.ValidMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
			So(patternsStorage.metrics.MatchingMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
		})

		Convey("When value has dot", func() {
			patternsStorage.metrics = metrics.NewFilterMetrics()
			patternsStorage.metrics.TotalMetricsReceived.Flush()

			for _, metric := range matchingMetrics {
				matchedMetrics := patternsStorage.ProcessIncomingMetric([]byte(metric + " 12.000000 1234567890"))
				So(matchedMetrics, ShouldNotBeNil)
			}
			So(patternsStorage.metrics.TotalMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
			So(patternsStorage.metrics.ValidMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
			So(patternsStorage.metrics.MatchingMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
		})

		Convey("When timestamp is float64", func() {
			patternsStorage.metrics = metrics.NewFilterMetrics()
			patternsStorage.metrics.TotalMetricsReceived.Flush()

			for _, metric := range matchingMetrics {
				matchedMetrics := patternsStorage.ProcessIncomingMetric([]byte(metric + " 12 1234567890.0"))
				So(matchedMetrics, ShouldNotBeNil)
			}
			So(patternsStorage.metrics.TotalMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
			So(patternsStorage.metrics.ValidMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
			So(patternsStorage.metrics.MatchingMetricsReceived.GetCount(), ShouldEqual, len(matchingMetrics))
		})
	})

	mockCtrl.Finish()
}
