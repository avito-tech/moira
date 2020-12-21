package heartbeat

import (
	"time"

	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
)

// Worker is heartbeat worker realization
type Worker struct {
	database  moira.Database
	logger    moira.Logger
	heartbeat chan bool
	tomb      tomb.Tomb
}

// NewHeartbeatWorker creates new worker
func NewHeartbeatWorker(database moira.Database, logger moira.Logger, heartbeat chan bool) *Worker {
	return &Worker{
		database:  database,
		logger:    logger,
		heartbeat: heartbeat,
	}
}

// Start every 5 second takes TotalMetricsReceived metrics and save it to database, for self-checking
func (worker *Worker) Start() {
	var newCount, oldCount int
	worker.tomb.Go(func() error {
		checkTicker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-worker.tomb.Dying():
				worker.logger.Info("Moira Filter Heartbeat stopped")
				return nil
			case <-worker.heartbeat:
				newCount++
			case <-checkTicker.C:
				newCount := newCount
				worker.logger.DebugF("Update heartbeat count, old value: %v, new value: %v", oldCount, newCount)
				if newCount != oldCount {
					if err := worker.database.UpdateMetricsHeartbeat(); err != nil {
						worker.logger.InfoF("Save state failed: %s", err.Error())
					} else {
						oldCount = newCount
					}
				}
			}
		}
	})

	worker.logger.Info("Moira Filter Heartbeat started")
}

// Stop heartbeat worker
func (worker *Worker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}
