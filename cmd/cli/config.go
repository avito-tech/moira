package main

import (
	"go.avito.ru/DO/moira/cmd"
)

type config struct {
	LogFile  string            `yaml:"log_file"`
	LogLevel string            `yaml:"log_level"`
	Redis    cmd.RedisConfig   `yaml:"redis"`
	Rsyslog  cmd.RsyslogConfig `yaml:"rsyslog"`
}

func getDefault() config {
	return config{
		LogFile:  "stdout",
		LogLevel: "debug",
		Redis: cmd.RedisConfig{
			Host: "localhost",
			Port: "6379",
			DBID: 0,
		},
		Rsyslog: cmd.RsyslogConfig{
			Enabled:  false,
			Host:     "127.0.0.1",
			Port:     514,
			Level:    "debug",
			Fallback: "stdout",
		},
	}
}
