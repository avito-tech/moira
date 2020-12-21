package logging

import (
	"fmt"
	"math/big"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging/go-logging"
	"go.avito.ru/DO/moira/metrics"
)

type Config struct {
	Enabled  bool
	Host     string
	Port     int
	Level    string
	Fallback string
	Debug    bool
}

type Logger struct {
	appComponent string
	callCounter  int
	contextID    ContextID
	config       *loggerConfig
}

func Init(appComponent string, config Config, rateLimit moira.RateLimit) error {
	initMu.Lock()
	defer initMu.Unlock()

	if cfg != nil {
		return ErrAlreadyInitialized{}
	}

	hostname, err := os.Hostname()
	if err != nil {
		return ErrFailedInitialize{reason: err}
	}

	level, weight := parseLogLevel(config.Level)
	cfg = &loggerConfig{
		app:     appComponent,
		debug:   config.Debug,
		enabled: config.Enabled,
		limits:  rateLimit,
		host:    config.Host,
		port:    config.Port,
		this:    hostname,
		level:   level,
		weight:  weight,
	}

	fallback, _ = logging.ConfigureLog(config.Fallback, level, appComponent)
	loggersStore = newStore()
	statsd = metrics.NewLoggerMetric()

	worker, err = newLoggingWorker()
	if err != nil {
		return ErrFailedInitialize{reason: err}
	}
	go worker.lifeCycle()

	return nil
}

// GetLogger returns logger instance for given id
// id must be either uuid or empty string
func GetLogger(id string) *Logger {
	if cfg == nil || loggersStore == nil {
		panic(ErrNotInitialized{})
	}

	return loggersStore.getOrCreate(id)
}

// GetLoggerSafe is equivalent to GetLogger
// but returns an error instead of panic
// if something goes wrong
func GetLoggerSafe(id string) (logger *Logger, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to get logger: panic = %v", r)
		}
	}()

	logger = GetLogger(id)
	return
}

func (logger *Logger) Debug(message string) {
	logger.log(logLevelDebug, message, nil)
}

func (logger *Logger) DebugE(message string, extra interface{}) {
	logger.log(logLevelDebug, message, extra)
}

func (logger *Logger) DebugF(format string, args ...interface{}) {
	logger.log(logLevelDebug, fmt.Sprintf(format, args...), nil)
}

func (logger *Logger) Info(message string) {
	logger.log(logLevelInfo, message, nil)
}

func (logger *Logger) InfoE(message string, extra interface{}) {
	logger.log(logLevelInfo, message, extra)
}

func (logger *Logger) InfoF(format string, args ...interface{}) {
	logger.log(logLevelInfo, fmt.Sprintf(format, args...), nil)
}

func (logger *Logger) Warn(message string) {
	logger.log(logLevelWarn, message, nil)
}

func (logger *Logger) WarnE(message string, extra interface{}) {
	logger.log(logLevelWarn, message, extra)
}

func (logger *Logger) WarnF(format string, args ...interface{}) {
	logger.log(logLevelWarn, fmt.Sprintf(format, args...), nil)
}

func (logger *Logger) Error(message string) {
	logger.log(logLevelError, message, nil)
}

func (logger *Logger) ErrorE(message string, extra interface{}) {
	logger.log(logLevelError, message, extra)
}

func (logger *Logger) ErrorF(format string, args ...interface{}) {
	logger.log(logLevelError, fmt.Sprintf(format, args...), nil)
}

func (logger *Logger) Fatal(message string) {
	logger.log(logLevelFatal, message, nil)
	os.Exit(1)
}

func (logger *Logger) FatalE(message string, extra interface{}) {
	logger.log(logLevelFatal, message, extra)
	os.Exit(1)
}

func (logger *Logger) FatalF(format string, args ...interface{}) {
	logger.log(logLevelFatal, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

func (logger *Logger) TracePanic(message string, extra interface{}) {
	logger.ErrorE(message, extra)
	useFallbackLogger(logLevelError, message, extra)
}

func (logger *Logger) TraceSelfStats(id string, started time.Time) {
	margin := moira.TriggerCheckThreshold * time.Second
	passed := time.Since(started)

	log := newInstance("self-stats")
	msg := fmt.Sprintf("Total log calls for id %s: %d", id, logger.callCounter)
	extra := map[string]interface{}{
		"calls":    logger.callCounter,
		"duration": passed.String(),
		"id":       id,
	}

	if passed < margin {
		log.InfoE(msg, extra)
	} else {
		log.ErrorE(msg, extra)
	}
}

func (logger *Logger) log(level, message string, extra interface{}) {
	logger.callCounter++
	worker.queue <- &logDelayedMessage{
		component: logger.appComponent,
		contextId: logger.contextID,
		dateTime:  time.Now(),
		level:     level,
		message:   message,
		path:      formatPath(3),
		extraData: extra,
	}
}

func (logger *Logger) setContextID(contextID uint64) {
	logger.contextID = ContextID(contextID)
}

func (logger *Logger) setContextIDFromUUID(uuidRaw string) {
	// remove all dashes and prepend leading zero if needed
	uuid := strings.Replace(uuidRaw, "-", "", -1)
	if len(uuid) < 32 {
		uuid = strings.Repeat("0", 32-len(uuid)) + uuid
	}

	// split uuid to 2 equal parts and parse them as integers
	x1, _ := big.NewInt(0).SetString(uuid[:16], 16)
	x2, _ := big.NewInt(0).SetString(uuid[16:], 16)

	// 2 ** 64
	mod := big.NewInt(2)
	mod.Exp(mod, big.NewInt(64), nil)

	// (x1 + x2) % (2 ** 64)
	x3 := big.NewInt(0)
	x3.Add(x1, x2)
	x3.Mod(x3, mod)

	if !x3.IsUint64() {
		panic(fmt.Sprintf("Could not parse %s as integer", uuidRaw))
	} else {
		logger.setContextID(x3.Uint64())
	}
}

func formatPath(skipFrames int) string {
	pc, file, line, _ := runtime.Caller(skipFrames)
	_, fileName := path.Split(file)
	parts := strings.Split(runtime.FuncForPC(pc).Name(), ".")
	partsTotal := len(parts)

	packageName := ""
	funcName := parts[partsTotal-1]

	if parts[partsTotal-2][0] == '(' {
		funcName = parts[partsTotal-2] + "." + funcName
		packageName = strings.Join(parts[0:partsTotal-2], ".")
	} else {
		packageName = strings.Join(parts[0:partsTotal-1], ".")
	}

	return fmt.Sprintf("[%s] %s: %s#%d", packageName, fileName, funcName, line)
}

func newInstance(id string) *Logger {
	logger := &Logger{
		appComponent: cfg.app,
		config:       cfg,
	}

	if id == "" {
		logger.setContextID(defaultContextId)
	} else if id == "panic" {
		logger.setContextID(panicContextId)
	} else if id == "self-stats" {
		logger.setContextID(selfStatsContextId)
	} else {
		logger.setContextIDFromUUID(id)
	}

	return logger
}
