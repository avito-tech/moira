package test_helpers

import (
	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
)

var (
	scheduleDaysAll = []moira.ScheduleDataDay{
		{true, "Mon"},
		{true, "Tue"},
		{true, "Wed"},
		{true, "Thu"},
		{true, "Fri"},
		{true, "Sat"},
		{true, "Sun"},
	}
	scheduleDaysNone = []moira.ScheduleDataDay{
		{false, "Mon"},
		{false, "Tue"},
		{false, "Wed"},
		{false, "Thu"},
		{false, "Fri"},
		{false, "Sat"},
		{false, "Sun"},
	}
)

func GetSchedule24x7() *moira.ScheduleData {
	return &moira.ScheduleData{
		Days:           scheduleDaysAll,
		TimezoneOffset: 0,
		StartOffset:    0,
		EndOffset:      1439,
	}
}

func GetScheduleNever() *moira.ScheduleData {
	return &moira.ScheduleData{
		Days:           scheduleDaysNone,
		TimezoneOffset: 0,
		StartOffset:    0,
		EndOffset:      1439,
	}
}

// GetTestLogger returns context unaware logger for tests purposes only
func GetTestLogger() *logging.Logger {
	InitTestLogging()
	return logging.GetLogger("")
}

// InitTestLogging initializes logging subsystem for tests purposes only
// syslog is disabled, fallback is stdout
func InitTestLogging() {
	rateLimits := moira.RateLimit{
		AcceptRate: 1,
		ThreadsQty: 2,
	}
	_ = metrics.Init(metrics.Config{Enabled: false, IsTest: true}, rateLimits)
	_ = logging.Init(
		logging.ComponentTests,
		logging.Config{
			Enabled:  false,
			Host:     "",
			Port:     0,
			Level:    "debug",
			Fallback: "stdout",
			Debug:    true,
		},
		rateLimits,
	)
}
