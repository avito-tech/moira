package logging

import (
	"fmt"
	"log/syslog"
	"sync"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging/go-logging"
	"go.avito.ru/DO/moira/metrics"
)

type ContextID uint64

const (
	ComponentApi      = "api"
	ComponentCli      = "cli"
	ComponentChecker  = "checker"
	ComponentFilter   = "filter"
	ComponentNotifier = "notifier"
	ComponentTests    = "tests"
)

const (
	defaultContextId   = uint64(1019682036619477495)  // some uint64 that is used as context id by default
	panicContextId     = uint64(5577006791947779410)  // some uint64 that is used as context id for panic recovery
	selfStatsContextId = uint64(13206152207838202834) // some uint64 that is used as context id for self-stats logging

	logLevelDebug   = "debug"
	logLevelInfo    = "info"
	logLevelWarn    = "warn"
	logLevelError   = "error"
	logLevelFatal   = "fatal"
	logLevelDefault = "info"

	facilityPriority = int(syslog.LOG_LOCAL1 | syslog.LOG_INFO)
)

// logDelayedMessage is struct for enqueuing log message
type logDelayedMessage struct {
	component string
	contextId ContextID
	dateTime  time.Time
	level     string
	message   string
	path      string
	extraData interface{}
}

// logEntry is struct prepared for serialization
type logEntry struct {
	Component     string    `json:"component"`
	ContextID     ContextID `json:"context_id"`
	EventDateTime string    `json:"event_datetime"`
	EventDate     string    `json:"event_date"`
	Extra         string    `json:"extra"`
	Level         string    `json:"level"`
	Message       string    `json:"msg"`
	Path          string    `json:"path"`
}

// loggerConfig is basic logger configuration, applied for all new loggers
type loggerConfig struct {
	app     string
	debug   bool
	enabled bool
	limits  moira.RateLimit
	host    string
	port    int
	this    string
	level   string
	weight  int
}

var (
	cfg    *loggerConfig
	worker *loggingWorker

	fallback *logging.Logger
	statsd   *metrics.LoggerMetrics

	initMu sync.Mutex
)

var (
	loggersStore    *store
	logLevelWeights = map[string]int{
		logLevelDebug: 10,
		logLevelInfo:  20,
		logLevelWarn:  30,
		logLevelError: 40,
		logLevelFatal: 50,
	}
)

type ErrAlreadyInitialized struct{}
type ErrNotInitialized struct{}

type ErrFailedInitialize struct {
	reason error
}
type ErrFailedSerialize struct {
	reason error
}
type ErrFailedTransport struct {
	reason error
}
type ErrIncompleteWrite struct {
	written, total int
}

func (err ErrAlreadyInitialized) Error() string {
	return "Logging subsystem has already been initialized"
}

func (err ErrNotInitialized) Error() string {
	return "Logging subsystem hasn't been initialized yet"
}

func (err ErrFailedInitialize) Error() string {
	return fmt.Sprintf("Failed to initialize logging subsystem: %v", err.reason)
}

func (err ErrFailedSerialize) Error() string {
	return fmt.Sprintf("Failed to serialize log record: %v", err.reason)
}

func (err ErrFailedTransport) Error() string {
	return fmt.Sprintf("Failed to connect or to send data via syslog: %v", err.reason)
}

func (err ErrIncompleteWrite) Error() string {
	return fmt.Sprintf("Could only write %d of %d bytes", err.written, err.total)
}
