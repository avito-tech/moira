package sentry

import (
	"github.com/getsentry/sentry-go"
)

//
// expose some sentry entities
//

var (
	ConfigureScope = sentry.ConfigureScope
	Flush          = sentry.Flush
)

const (
	LevelDebug   = sentry.LevelDebug
	LevelInfo    = sentry.LevelInfo
	LevelWarning = sentry.LevelWarning
	LevelError   = sentry.LevelError
	LevelFatal   = sentry.LevelFatal
)

type Scope = sentry.Scope
