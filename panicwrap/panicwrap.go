package panicwrap

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mitchellh/panicwrap"

	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/sentry"
)

const timeout = 5 * time.Second

func Init(serviceName string) {
	code, err := panicwrap.BasicWrap(func(output string) {
		panicHandler(output, serviceName)
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to Init panicwrap: %v\n", err)
		os.Exit(1)
	}

	// BasicWrap call duplicates current process (like fork).
	// One copy continues execution immediately (main process), its code is negative.
	// The second copy waits until main process ends; its code is actual process status code (i.e. non-negative).
	if code >= 0 {
		os.Exit(code)
	}
}

// panicHandler is called whenever there is a panic that hasn't been recovered
// it is called with typical go panic output
// too bad it isn't panic object itself, but it is better then nothing
func panicHandler(output, serviceName string) {
	done := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(2)

	// trace to sentry
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		defer sentry.Flush(timeout)

		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelFatal)
		})
		_ = sentry.LogMessage(fmt.Sprintf("[%s] Unhandled panic", serviceName), output)
	}(&wg)

	// trace to syslog
	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		logger, err := logging.GetLoggerSafe("panic")
		if err != nil {
			return
		}
		logger.ErrorE("Unhandled panic", map[string]string{
			"output": output,
		})

		// since logger is asynchronous, give it some time to emit data
		time.Sleep(timeout)
	}(&wg)

	// keeps after previous 2 calls
	go func(wg *sync.WaitGroup) {
		wg.Wait()
		done <- true
	}(&wg)

	select {
	case <-done:
		// all traced
		os.Exit(1)
	case <-time.After(timeout + 100*time.Millisecond):
		// timeout
		os.Exit(2)
	}
}
