package worker

import (
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/checker"
	"go.avito.ru/DO/moira/metrics"
)

// Checker represents workers for periodically triggers checking based by new events
type Checker struct {
	Logger   moira.Logger
	Database moira.Database
	Config   *checker.Config
	Metrics  *metrics.CheckerMetrics
	Cache    *cache.Cache

	lastData            int64
	triggersToCheck     chan string
	pullTriggersToCheck chan string
	tagsToCheck         chan string

	tomb tomb.Tomb
}

// Start start schedule new MetricEvents and check for NODATA triggers
func (worker *Checker) Start() error {
	if worker.Config.MaxParallelChecks == 0 {
		return fmt.Errorf("MaxParallelChecks does not configure, checker does not started")
	}

	worker.lastData = time.Now().UTC().Unix()
	worker.triggersToCheck = make(chan string, 100)
	worker.pullTriggersToCheck = make(chan string, 100)
	worker.tagsToCheck = make(chan string, 100)

	metricEventsChannel, err := worker.Database.SubscribeMetricEvents(&worker.tomb)
	if err != nil {
		return err
	}

	worker.tomb.Go(worker.noDataChecker)
	worker.Logger.Info("NoData checker started")

	worker.tomb.Go(func() error {
		return worker.metricsChecker(metricEventsChannel)
	})
	worker.tomb.Go(worker.checkPullTriggers)
	worker.Logger.Info("Checking remote triggers started")

	worker.tomb.Go(worker.tagsChecker)
	worker.Logger.Info("Empty tags checker started")

	for i := 0; i < worker.Config.MaxParallelChecks; i++ {
		worker.tomb.Go(func() error {
			return worker.triggerHandler(worker.triggersToCheck)
		})
	}
	for i := 0; i < worker.Config.MaxParallelPullChecks; i++ {
		worker.tomb.Go(func() error {
			return worker.triggerHandler(worker.pullTriggersToCheck)
		})
	}
	for i := 0; i < worker.Config.MaxParallelTagsChecks; i++ {
		worker.tomb.Go(func() error {
			return worker.tagsHandler(worker.tagsToCheck)
		})
	}

	worker.Logger.InfoF("Start %v parallel checkers", worker.Config.MaxParallelChecks)
	worker.Logger.InfoF("Start %v parallel pull checkers", worker.Config.MaxParallelPullChecks)
	worker.Logger.InfoF("Start %v parallel empty tags checkers", worker.Config.MaxParallelTagsChecks)

	worker.Logger.Info("Checking new events started")
	return nil
}

func (worker *Checker) checkPullTriggers() error {
	checkTicker := time.NewTicker(worker.Config.PullInterval)
	for {
		select {
		case <-worker.tomb.Dying():
			close(worker.pullTriggersToCheck)
			worker.Logger.Info("pull checker stopped")
			return nil
		case <-checkTicker.C:
			triggerIDs, err := worker.Database.GetTriggerIDs(true)
			if err != nil {
				worker.Logger.ErrorF("pull worker error: %v", err)
				continue
			}

			worker.Logger.InfoF("pulled %d trigger(s)", len(triggerIDs))
			worker.perform(triggerIDs, worker.Config.CheckInterval, true)
		}
	}
}

// Stop stops checks triggers
func (worker *Checker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}
