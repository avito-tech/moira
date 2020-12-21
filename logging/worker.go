package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aristanetworks/goarista/monotime"
	"gopkg.in/tomb.v2"
)

const (
	bufferSize = 65536
)

type loggingWorker struct {
	conn  []net.Conn
	queue chan *logDelayedMessage
	tomb  tomb.Tomb
}

func newLoggingWorker() (*loggingWorker, error) {
	var (
		conn = make([]net.Conn, cfg.limits.ThreadsQty)
		err  error
	)

	if cfg.enabled {
		for i := 0; i < cfg.limits.ThreadsQty; i++ {
			conn[i], err = dial()
			if err != nil {
				return nil, ErrFailedTransport{reason: err}
			}
		}
	}

	result := &loggingWorker{
		conn:  conn,
		queue: make(chan *logDelayedMessage, bufferSize),
	}
	return result, nil
}

func (worker *loggingWorker) consumer(id int) error {
	var (
		delayedMessage *logDelayedMessage
		skip           bool
		level          string
		weight         int

		data  []byte
		err   error
		extra string
	)

	for {
		select {
		case <-worker.tomb.Dying():
			return nil
		case delayedMessage = <-worker.queue:
			// pass
		}

		// level check
		level, weight = parseLogLevel(delayedMessage.level)
		if weight < cfg.weight {
			continue
		}

		// rate limit validation
		skip = !cfg.enabled || (cfg.limits.AcceptRate != 1 && cfg.limits.AcceptRate < rand.Float64())
		if cfg.debug || skip {
			useFallbackLogger(level, delayedMessage.message, delayedMessage.extraData)
			if skip {
				continue
			}
		}

		// it is needed 2 formats of dateTime
		dateTimeRFC := delayedMessage.dateTime.Format("2006-01-02T15:04:05.999999999")
		dateTimeStamp := delayedMessage.dateTime.Format(time.Stamp)

		// serialized extra data if provided
		extra = ""
		if delayedMessage.extraData != nil {
			data, err = json.Marshal(delayedMessage.extraData)
			if err != nil {
				handleError(delayedMessage.message, ErrFailedSerialize{reason: err})
				continue
			}
			extra = string(data)
		}

		// serialize full log message
		entry := logEntry{
			Component:     delayedMessage.component,
			ContextID:     delayedMessage.contextId,
			EventDateTime: dateTimeRFC,
			EventDate:     dateTimeRFC[:10],
			Extra:         extra,
			Level:         level,
			Message:       delayedMessage.message,
			Path:          delayedMessage.path,
		}
		data, err = json.Marshal(entry)
		if err != nil {
			handleError(delayedMessage.message, ErrFailedSerialize{reason: err})
			continue
		}

		// write to buffer and send to connection
		buffer := bytes.Buffer{}
		buffer.Grow(len(data) + 128)
		buffer.WriteByte('<')
		buffer.WriteString(strconv.Itoa(facilityPriority))
		buffer.WriteByte('>')
		buffer.WriteString(dateTimeStamp)
		buffer.WriteByte(' ')
		buffer.WriteString(cfg.this)
		buffer.WriteString(" moira: ")
		buffer.Write(data)
		buffer.WriteByte('\n')

		data = buffer.Bytes()
		statsd.MsgSize.Timing(int64(len(data)))

		err = worker.write(id, data)
		if err != nil {
			handleError(delayedMessage.message, ErrFailedTransport{reason: err})
			continue
		}
	}
}

func (worker *loggingWorker) lifeCycle() {
	worker.start()
	defer worker.stop()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func (worker *loggingWorker) start() {
	for i := 0; i < cfg.limits.ThreadsQty; i++ {
		func(id int) {
			worker.tomb.Go(func() error {
				return worker.consumer(id)
			})
		}(i)
	}
}

func (worker *loggingWorker) stop() {
	worker.tomb.Kill(nil)
	_ = worker.tomb.Wait()
}

// write sends the whole message body
func (worker *loggingWorker) write(id int, data []byte) error {
	var (
		conn      net.Conn
		started   uint64
		reconnect uint64
		n, total  int
		err       error
	)

	total = len(data)
	conn = worker.conn[id]
	started = monotime.Now()

	defer func() {
		statsd.MsgTotal.Increment()
		statsd.Write.Timing(int64(monotime.Since(started)))
	}()

	// send data in one piece
	n, err = conn.Write(data)
	if err != nil {
		// try to redial and retry writing in case of error
		reconnect = monotime.Now()
		conn, err = dial()
		statsd.Reconnect.Timing(int64(monotime.Since(reconnect)))
		if err != nil {
			return err
		}

		// replace connection if redial was successful
		worker.conn[id] = conn
		n, err = conn.Write(data)
	}

	if err != nil {
		return err
	}
	if n != total {
		return ErrIncompleteWrite{written: n, total: total}
	}

	return nil
}

func dial() (net.Conn, error) {
	return net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.host, cfg.port))
}

func handleError(message string, err error) {
	useFallbackLogger(logLevelError, fmt.Sprintf("Failed to log: %v", err), nil)
	useFallbackLogger(logLevelError, fmt.Sprintf("Message was:\n%s", message), nil)

	if statsd != nil {
		statsd.Errors.Increment()
	}
}

func parseLogLevel(level string) (string, int) {
	weight, ok := logLevelWeights[level]
	if !ok {
		level = logLevelDefault
		weight = logLevelWeights[level]
	}
	return level, weight
}

func useFallbackLogger(level, message string, extra interface{}) {
	if extra != nil { // try to serialize extra
		var (
			extraStr string
			success  bool
		)

		if cfg.debug { // pretty print for debug mode
			extraBytes, err := json.MarshalIndent(extra, "", "  ")
			if err == nil {
				extraStr = string(extraBytes)
				success = true
			}
		}
		if !success {
			extraStr = fmt.Sprintf("%v", extra)
		}

		message = fmt.Sprintf("%s\nExtra: %s", message, extraStr)
	}

	if level == logLevelDebug {
		fallback.Debug(message)
	} else if level == logLevelInfo {
		fallback.Info(message)
	} else if level == logLevelWarn {
		fallback.Warning(message)
	} else if level == logLevelError {
		fallback.Error(message)
	} else if level == logLevelFatal {
		fallback.Fatal(message)
	}
}
