package main

import (
	"time"

	"github.com/gosexy/to"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/cmd"
	"go.avito.ru/DO/moira/notifier"
	"go.avito.ru/DO/moira/notifier/selfstate"
)

type config struct {
	Redis    cmd.RedisConfig    `yaml:"redis"`
	Neo4j    cmd.Neo4jConfig    `yaml:"neo4j"`
	Logger   cmd.LoggerConfig   `yaml:"log"`
	Rsyslog  cmd.RsyslogConfig  `yaml:"rsyslog"`
	Statsd   cmd.StatsdConfig   `yaml:"statsd"`
	Notifier notifierConfig     `yaml:"notifier"`
	Pprof    cmd.ProfilerConfig `yaml:"pprof"`
	Liveness cmd.LivenessConfig `yaml:"liveness"`
}

type notifierConfig struct {
	SenderTimeout    string              `yaml:"sender_timeout"`
	ResendingTimeout string              `yaml:"resending_timeout"`
	Senders          []map[string]string `yaml:"senders"`
	SelfState        selfStateConfig     `yaml:"moira_selfstate"`
	Sentry           cmd.SentryConfig    `yaml:"sentry"`
	FrontURI         string              `yaml:"front_uri"`
	Timezone         string              `yaml:"timezone"`
	LimitLogger      cmd.RateLimit       `yaml:"limit_logger"`
	LimitMetrics     cmd.RateLimit       `yaml:"limit_metrics"`
	DutyApiToken     string              `yaml:"duty_api_token"`
	DutyUrl          string              `yaml:"duty_url"`
	FanURL           string              `yaml:"fan_url"`
}

type selfStateConfig struct {
	Enabled                 bool                `yaml:"enabled"`
	RedisDisconnectDelay    string              `yaml:"redis_disconect_delay"`
	LastMetricReceivedDelay string              `yaml:"last_metric_received_delay"`
	LastCheckDelay          string              `yaml:"last_check_delay"`
	Contacts                []map[string]string `yaml:"contacts"`
	NoticeInterval          string              `yaml:"notice_interval"`
}

func getDefault() config {
	return config{
		Redis: cmd.RedisConfig{
			Host: "localhost",
			Port: "6379",
			DBID: 0,
		},
		Neo4j: cmd.Neo4jConfig{
			Host:     "neo4j",
			Port:     7474,
			DBName:   "neo4j",
			User:     "neo4j",
			Password: "neo4j",
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
		Logger: cmd.LoggerConfig{
			LogFile:  "stdout",
			LogLevel: "debug",
		},
		Notifier: notifierConfig{
			SenderTimeout:    "10s",
			ResendingTimeout: "24:00",
			SelfState: selfStateConfig{
				Enabled:                 false,
				RedisDisconnectDelay:    "30s",
				LastMetricReceivedDelay: "60s",
				LastCheckDelay:          "60s",
				NoticeInterval:          "300s",
			},
			Sentry: cmd.SentryConfig{
				Dsn:     "",
				Enabled: false,
			},
			FrontURI:     "http://localhost",
			Timezone:     "UTC",
			LimitLogger:  cmd.NewDefaultLoggerRateLimit(),
			LimitMetrics: cmd.NewDefaultMetricsRateLimit(),
			DutyApiToken: "",
			DutyUrl:      "",
			FanURL:       "http://localhost:3260/api",
		},
		Pprof: cmd.ProfilerConfig{
			Listen: "",
		},
		Liveness: cmd.LivenessConfig{
			Listen: "",
		},
	}
}

func (config *notifierConfig) getSettings(logger moira.Logger) notifier.Config {
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		logger.WarnF("Timezone '%s' load failed: %s. Use UTC.", config.Timezone, err.Error())
		location, _ = time.LoadLocation("UTC")
	} else {
		logger.InfoF("Timezone '%s' loaded.", config.Timezone)
	}

	return notifier.Config{
		SendingTimeout:   to.Duration(config.SenderTimeout),
		ResendingTimeout: to.Duration(config.ResendingTimeout),
		Senders:          config.Senders,
		FrontURL:         config.FrontURI,
		Location:         location,
		DutyApiToken:     config.DutyApiToken,
		DutyUrl:          config.DutyUrl,
	}
}

func (config *selfStateConfig) getSettings() selfstate.Config {
	return selfstate.Config{
		Enabled:                        config.Enabled,
		RedisDisconnectDelaySeconds:    int64(to.Duration(config.RedisDisconnectDelay).Seconds()),
		LastMetricReceivedDelaySeconds: int64(to.Duration(config.LastMetricReceivedDelay).Seconds()),
		LastCheckDelaySeconds:          int64(to.Duration(config.LastCheckDelay).Seconds()),
		Contacts:                       config.Contacts,
		NoticeIntervalSeconds:          int64(to.Duration(config.NoticeInterval).Seconds()),
	}
}
