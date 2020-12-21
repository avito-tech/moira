package matchedmetrics

import (
	"sync"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/filter"
	"go.avito.ru/DO/moira/metrics"
)

const (
	cacheCapacity = 1000
	flushInterval = 5 * time.Second
)

// MetricsMatcher make buffer of metrics and save it
type MetricsMatcher struct {
	logger       moira.Logger
	metrics      *metrics.FilterMetrics
	database     moira.Database
	cacheStorage *filter.Storage
	waitGroup    *sync.WaitGroup
}

// NewMetricsMatcher creates new MetricsMatcher
func NewMetricsMatcher(metrics *metrics.FilterMetrics, logger moira.Logger, database moira.Database, cacheStorage *filter.Storage) *MetricsMatcher {
	return &MetricsMatcher{
		metrics:      metrics,
		logger:       logger,
		database:     database,
		cacheStorage: cacheStorage,
		waitGroup:    &sync.WaitGroup{},
	}
}

// Start process matched metrics from channel and save it in cache storage
func (matcher *MetricsMatcher) Start(channel chan *moira.MatchedMetric) {
	matcher.waitGroup.Add(1)
	go func() {
		defer matcher.waitGroup.Done()

		buffer := make(map[string]*moira.MatchedMetric, cacheCapacity)
		for {
			select {
			case metric, ok := <-channel:
				if !ok {
					matcher.logger.Info("Moira Filter Metrics Matcher stopped")
					return
				}
				matcher.cacheStorage.EnrichMatchedMetric(buffer, metric)
				if len(buffer) < cacheCapacity {
					continue
				}
			case <-time.After(flushInterval):
			}

			if len(buffer) == 0 {
				continue
			}

			timer := time.Now()
			matcher.save(buffer)
			matcher.metrics.SavingTimer.UpdateSince(timer)
			buffer = make(map[string]*moira.MatchedMetric, cacheCapacity)
		}
	}()
	matcher.logger.Info("Moira Filter Metrics Matcher started")
}

// Wait waits for metric matcher instance will stop
func (matcher *MetricsMatcher) Wait() {
	matcher.waitGroup.Wait()
}

func (matcher *MetricsMatcher) save(buffer map[string]*moira.MatchedMetric) {
	if err := matcher.database.SaveMetrics(buffer); err != nil {
		matcher.logger.InfoF("Failed to save value in cache storage: %s", err.Error())
	}
}
