package cmd

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/netbox"
	"go.avito.ru/DO/moira/sentry"
)

// RedisConfig is redis config structure, which are taken on the start of moira
type RedisConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	DBID int    `yaml:"dbid"`
}

// GetSettings return redis config parsed from moira config files
func (config *RedisConfig) GetSettings() redis.Config {
	return redis.Config{
		Host: config.Host,
		Port: config.Port,
		DBID: config.DBID,
	}
}

type Neo4jConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	DBName       string `yaml:"db_name"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	PasswordPath string `yaml:"password_path"`
}

// LoggerConfig is logger settings, which are taken on the start of moira
type LoggerConfig struct {
	LogFile  string `yaml:"log_file"`
	LogLevel string `yaml:"log_level"`
}

// NetboxConfig is settings of netbox client, which are taken on the start of moira
type NetboxConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
	URL     string `yaml:"url"`
}

func (netboxConfig *NetboxConfig) GetSettings() *netbox.Config {
	if !netboxConfig.Enabled {
		return nil
	}

	token, _ := moira.GetFileContent(netboxConfig.Token)
	url := strings.TrimSuffix(netboxConfig.URL, "/")
	return &netbox.Config{
		Token: strings.TrimSpace(token),
		URL:   url,
	}
}

type RateLimit struct {
	AcceptRate float64 `yaml:"rate"`
	ThreadsQty int     `yaml:"threads"`
}

func NewDefaultLoggerRateLimit() RateLimit {
	return RateLimit{
		AcceptRate: 1,
		ThreadsQty: 4,
	}
}

func NewDefaultMetricsRateLimit() RateLimit {
	return RateLimit{
		AcceptRate: 1,
		ThreadsQty: 4,
	}
}

func (rateLimit *RateLimit) GetSettings() moira.RateLimit {
	var (
		acceptRate = rateLimit.AcceptRate
		threadsQty = rateLimit.ThreadsQty
	)

	if acceptRate == 0 {
		acceptRate = 1
	}

	if threadsQty == 0 {
		threadsQty = 8
	}

	return moira.RateLimit{
		AcceptRate: acceptRate,
		ThreadsQty: threadsQty,
	}
}

type RsyslogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Level    string `yaml:"level"`
	Fallback string `yaml:"fallback"`
	Debug    bool   `yaml:"debug"`
}

func (rsyslogConfig *RsyslogConfig) GetSettings() logging.Config {
	return logging.Config{
		Enabled:  rsyslogConfig.Enabled,
		Host:     rsyslogConfig.Host,
		Port:     rsyslogConfig.Port,
		Level:    rsyslogConfig.Level,
		Fallback: rsyslogConfig.Fallback,
		Debug:    rsyslogConfig.Debug,
	}
}

type StatsdConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Prefix  string `yaml:"prefix"`
}

func (statsdConfig *StatsdConfig) GetSettings() metrics.Config {
	return metrics.Config{
		Enabled: statsdConfig.Enabled,
		Host:    statsdConfig.Host,
		Port:    statsdConfig.Port,
		Prefix:  statsdConfig.Prefix,
	}
}

// ProfilerConfig is pprof settings, which are taken on the start of moira
type ProfilerConfig struct {
	Listen string `yaml:"listen"`
}

// LivenessConfig is liveness check settings, which are taken on the start of moira
type LivenessConfig struct {
	Listen string `yaml:"listen"`
}

// SentryConfig is configuration for sentry reporter
type SentryConfig struct {
	Dsn        string `yaml:"dsn"`
	Enabled    bool   `yaml:"enabled"`
	IsFilePath bool   `yaml:"is_file_path"`
}

func (sentryConfig *SentryConfig) GetSettings() sentry.Config {
	return sentry.Config{
		Dsn:        sentryConfig.Dsn,
		Enabled:    sentryConfig.Enabled,
		IsFilePath: sentryConfig.IsFilePath,
	}
}

// ReadConfig gets config file by given file and marshal it to moira-used type
func ReadConfig(configFileName string, config interface{}) error {
	configYaml, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return fmt.Errorf("Can't read file [%s] [%s]", configFileName, err.Error())
	}
	err = yaml.Unmarshal(configYaml, config)
	if err != nil {
		return fmt.Errorf("Can't parse config file [%s] [%s]", configFileName, err.Error())
	}
	return nil
}

// PrintConfig prints config to std
func PrintConfig(config interface{}) {
	d, _ := yaml.Marshal(&config)
	fmt.Println(string(d))
}
