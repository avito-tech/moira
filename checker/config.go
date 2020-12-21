package checker

import (
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/netbox"
	"go.avito.ru/DO/moira/sentry"
)

// Config represent checker config
type Config struct {
	Enabled                     bool
	NoDataCheckInterval         time.Duration
	TagsCheckInterval           time.Duration
	CheckInterval               time.Duration
	PullInterval                time.Duration
	PullURL                     string
	MetricsTTLSeconds           int64
	StopCheckingIntervalSeconds int64
	MaxParallelChecks           int
	MaxParallelPullChecks       int
	MaxParallelTagsChecks       int
	LogFile                     string
	LogLevel                    string
	Netbox                      *netbox.Config
	LimitLogger                 moira.RateLimit
	LimitMetrics                moira.RateLimit
	Sentry                      sentry.Config
}
