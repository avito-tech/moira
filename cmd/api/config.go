package main

import (
	"strings"

	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/cmd"
)

type config struct {
	API      apiConfig          `yaml:"api"`
	Redis    cmd.RedisConfig    `yaml:"redis"`
	Neo4j    cmd.Neo4jConfig    `yaml:"neo4j"`
	Liveness cmd.LivenessConfig `yaml:"liveness"`
	Logger   cmd.LoggerConfig   `yaml:"log"`
	Netbox   cmd.NetboxConfig   `yaml:"netbox"`
	Pprof    cmd.ProfilerConfig `yaml:"pprof"`
	Rsyslog  cmd.RsyslogConfig  `yaml:"rsyslog"`
	Statsd   cmd.StatsdConfig   `yaml:"statsd"`
}

type apiConfig struct {
	Listen             string           `yaml:"listen"`
	EnableCORS         bool             `yaml:"enable_cors"`
	GrafanaPrefixes    []string         `yaml:"grafana_prefixes"`
	LimitLogger        cmd.RateLimit    `yaml:"limit_logger"`
	LimitMetrics       cmd.RateLimit    `yaml:"limit_metrics"`
	Sentry             cmd.SentryConfig `yaml:"sentry"`
	SuperUsers         []string         `yaml:"super_users"`
	TargetRewriteRules []rewriteRule    `yaml:"target_rewrite"`
	WebConfigPath      string           `yaml:"web_config_path"`
}

type rewriteRule struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func (config *apiConfig) getSettings() *api.Config {
	rewriteRules := make([]api.RewriteRule, len(config.TargetRewriteRules))
	for i, rule := range config.TargetRewriteRules {
		rewriteRules[i] = api.RewriteRule{
			From: rule.From,
			To:   rule.To,
		}
	}

	grafanaPrefixes := make([]string, 0, len(config.GrafanaPrefixes))
	for i := 0; i < len(config.GrafanaPrefixes); i++ {
		prefix := strings.TrimSpace(config.GrafanaPrefixes[i])
		if prefix != "" {
			grafanaPrefixes = append(grafanaPrefixes, prefix)
		}
	}

	return &api.Config{
		EnableCORS:         config.EnableCORS,
		GrafanaPrefixes:    grafanaPrefixes,
		Listen:             config.Listen,
		LimitLogger:        config.LimitLogger.GetSettings(),
		LimitMetrics:       config.LimitMetrics.GetSettings(),
		Sentry:             config.Sentry.GetSettings(),
		SuperUsers:         config.SuperUsers,
		TargetRewriteRules: rewriteRules,
	}
}

func getDefault() config {
	return config{
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
		Netbox: cmd.NetboxConfig{},
		Logger: cmd.LoggerConfig{
			LogFile:  "stdout",
			LogLevel: "debug",
		},
		API: apiConfig{
			Listen:          ":8081",
			LimitLogger:     cmd.NewDefaultLoggerRateLimit(),
			LimitMetrics:    cmd.NewDefaultMetricsRateLimit(),
			WebConfigPath:   "/etc/moira/web.json",
			EnableCORS:      false,
			GrafanaPrefixes: []string{},
			Sentry: cmd.SentryConfig{
				Dsn:     "",
				Enabled: false,
			},
			SuperUsers:         []string{},
			TargetRewriteRules: []rewriteRule{},
		},
		Pprof: cmd.ProfilerConfig{
			Listen: "",
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
		Liveness: cmd.LivenessConfig{
			Listen: "",
		},
	}
}
