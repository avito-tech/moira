package main

import (
	"go.avito.ru/DO/moira/cmd"
)

type config struct {
	Redis    cmd.RedisConfig    `yaml:"redis"`
	Logger   cmd.LoggerConfig   `yaml:"log"`
	Rsyslog  cmd.RsyslogConfig  `yaml:"rsyslog"`
	Statsd   cmd.StatsdConfig   `yaml:"statsd"`
	Filter   filterConfig       `yaml:"filter"`
	Pprof    cmd.ProfilerConfig `yaml:"pprof"`
	Liveness cmd.LivenessConfig `yaml:"liveness"`
}

type filterConfig struct {
	Listen            string           `yaml:"listen"`
	LimitLogger       cmd.RateLimit    `yaml:"limit_logger"`
	LimitMetrics      cmd.RateLimit    `yaml:"limit_metrics"`
	MaxParallelChecks int              `yaml:"max_parallel_checks"`
	RetentionConfig   string           `yaml:"retention-config"`
	Sentry            cmd.SentryConfig `yaml:"sentry"`
}

func getDefault() config {
	return config{
		Redis: cmd.RedisConfig{
			Host: "localhost",
			Port: "6379",
			DBID: 0,
		},
		Logger: cmd.LoggerConfig{
			LogFile:  "stdout",
			LogLevel: "debug",
		},
		Filter: filterConfig{
			Listen:          ":2003",
			LimitLogger:     cmd.NewDefaultLoggerRateLimit(),
			LimitMetrics:    cmd.NewDefaultMetricsRateLimit(),
			RetentionConfig: "/etc/moira/storage-schemas.conf",
			Sentry: cmd.SentryConfig{
				Dsn:     "",
				Enabled: false,
			},
		},
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
			Listen: "",
		},
		Liveness: cmd.LivenessConfig{
			Listen: "",
		},
	}
}
