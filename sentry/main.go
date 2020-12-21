package sentry

import (
	"fmt"
	"os"
	"sync"

	"github.com/getsentry/sentry-go"

	"go.avito.ru/DO/moira"
)

var (
	lock        sync.RWMutex
	enabled     bool
	initialized bool
)

func Init(config Config, serviceName string) error {
	lock.Lock()
	defer lock.Unlock()

	if initialized {
		return ErrAlreadyInit
	}

	var (
		dsn string
		err error
	)

	initialized = true
	defer func() {
		if err != nil {
			initialized = false
		}
	}()

	dsn = config.Dsn
	if dsn != "" && config.IsFilePath {
		dsn, err = moira.GetFileContent(dsn)
		if err != nil {
			return err
		}
	}

	enabled = config.Enabled && dsn != ""
	if !enabled {
		return nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	err = sentry.Init(sentry.ClientOptions{
		AttachStacktrace: true,
		Dsn:              dsn,
		ServerName:       hostname,
	})
	if err != nil {
		return err
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetExtra("service", serviceName)
		scope.SetTag("service", serviceName)
	})
	return nil
}

func LogMessage(header, body string) error {
	lock.RLock()
	enabled, initialized := enabled, initialized
	lock.RUnlock()

	if !initialized {
		return ErrNotInit
	}

	if enabled {
		sentry.CaptureMessage(fmt.Sprintf("%s\n%s", header, body))
	}

	return nil
}
