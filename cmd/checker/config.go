package main

import (
	"runtime"

	"github.com/gosexy/to"

	"go.avito.ru/DO/moira/checker"
	"go.avito.ru/DO/moira/cmd"
)

type config struct {
	Checker  checkerConfig      `yaml:"checker"`
	Redis    cmd.RedisConfig    `yaml:"redis"`
	Neo4j    cmd.Neo4jConfig    `yaml:"neo4j"`
	Logger   cmd.LoggerConfig   `yaml:"log"`
	Netbox   cmd.NetboxConfig   `yaml:"netbox"`
	Rsyslog  cmd.RsyslogConfig  `yaml:"rsyslog"`
	Statsd   cmd.StatsdConfig   `yaml:"statsd"`
	Pprof    cmd.ProfilerConfig `yaml:"pprof"`
	Liveness cmd.LivenessConfig `yaml:"liveness"`
}

type checkerConfig struct {
	CheckInterval         string           `yaml:"check_interval"`
	NoDataCheckInterval   string           `yaml:"nodata_check_interval"`
	TagsCheckInterval     string           `yaml:"tags_check_interval"`
	PullInterval          string           `yaml:"pull_interval"`
	PullURL               string           `yaml:"pull_url"`
	MetricsTTL            string           `yaml:"metrics_ttl"`
	StopCheckingInterval  string           `yaml:"stop_checking_interval"`
	MaxParallelChecks     int              `yaml:"max_parallel_checks"`
	MaxParallelPullChecks int              `yaml:"max_parallel_pull_checks"`
	MaxParallelTagsChecks int              `yaml:"max_parallel_tags_checks"`
	Sentry                cmd.SentryConfig `yaml:"sentry"`
	LimitLogger           cmd.RateLimit    `yaml:"limit_logger"`
	LimitMetrics          cmd.RateLimit    `yaml:"limit_metrics"`
}

func (config *checkerConfig) getSettings() *checker.Config {
	if config.MaxParallelChecks == 0 {
		config.MaxParallelChecks = runtime.NumCPU()
	}
	if config.MaxParallelPullChecks == 0 {
		config.MaxParallelPullChecks = runtime.NumCPU()
	}
	if config.MaxParallelTagsChecks == 0 {
		config.MaxParallelTagsChecks = runtime.NumCPU()
	}

	return &checker.Config{
		MetricsTTLSeconds:           int64(to.Duration(config.MetricsTTL).Seconds()),
		CheckInterval:               to.Duration(config.CheckInterval),
		NoDataCheckInterval:         to.Duration(config.NoDataCheckInterval),
		TagsCheckInterval:           to.Duration(config.TagsCheckInterval),
		PullInterval:                to.Duration(config.PullInterval),
		PullURL:                     config.PullURL,
		StopCheckingIntervalSeconds: int64(to.Duration(config.StopCheckingInterval).Seconds()),
		MaxParallelChecks:           config.MaxParallelChecks,
		MaxParallelPullChecks:       config.MaxParallelPullChecks,
		MaxParallelTagsChecks:       config.MaxParallelTagsChecks,
		LimitLogger:                 config.LimitLogger.GetSettings(),
		LimitMetrics:                config.LimitMetrics.GetSettings(),
		Sentry:                      config.Sentry.GetSettings(),
	}
}

func getDefault() config {
	return config{
		Checker: checkerConfig{
			CheckInterval:         "5s",
			NoDataCheckInterval:   "60s",
			TagsCheckInterval:     "1h",
			PullInterval:          "60s",
			PullURL:               "http://graphite/render/",
			MetricsTTL:            "1h",
			StopCheckingInterval:  "30s",
			MaxParallelChecks:     0,
			MaxParallelPullChecks: 0,
			MaxParallelTagsChecks: 0,
			Sentry: cmd.SentryConfig{
				Dsn:     "",
				Enabled: false,
			},
			LimitLogger:  cmd.NewDefaultLoggerRateLimit(),
			LimitMetrics: cmd.NewDefaultMetricsRateLimit(),
		},
		Redis: cmd.RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
		Neo4j: cmd.Neo4jConfig{
			Host:     "neo4j",
			Port:     7474,
			DBName:   "neo4j",
			User:     "neo4j",
			Password: "neo4j",
		},
		Logger: cmd.LoggerConfig{
			LogFile:  "stdout",
			LogLevel: "debug",
		},
		Netbox: cmd.NetboxConfig{},
		Rsyslog: cmd.RsyslogConfig{
			Enabled:  false,
			Host:     "127.0.0.1",
			Port:     514,
			Level:    "debug",
			Fallback: "stdout",
			Debug:    false,
		},
		Statsd: cmd.StatsdConfig{
			Enabled: false,
			Host:    "localhost",
			Port:    2003,
			Prefix:  "resources.monitoring.moira.localhost",
		},
		Pprof: cmd.ProfilerConfig{
			Listen: ":8900",
		},
		Liveness: cmd.LivenessConfig{
			Listen: "",
		},
	}
}
