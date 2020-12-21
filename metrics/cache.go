package metrics

import (
	"github.com/rcrowley/go-metrics"
)

const (
	sdDropped   = "statsd.calls.dropped"
	sdFlushTime = "statsd.time.emit"
	sdQueueSize = "statsd.size.queue"
)

// metricsCache keeps metrics.Meter and metrics.Timer to aggregate delayed calls before flushing them
type metricsCache struct {
	data     map[string]pair
	registry metrics.Registry

	// self-diagnose metrics
	droppedCalls pair
	queueSize    pair
}

type pair struct {
	// meter is metrics.Histogram because default metrics.Meter has timer that locks its mutex every 5 seconds
	meter     metrics.Histogram
	histogram metrics.Histogram
}

func newMetricsCache() *metricsCache {
	metricsCache := &metricsCache{
		data:     make(map[string]pair, 256),
		registry: metrics.NewRegistry(),
	}

	metricsCache.droppedCalls = metricsCache.getOrCreate(sdDropped)
	metricsCache.queueSize = metricsCache.getOrCreate(sdQueueSize)

	return metricsCache
}

// getOrCreate returns an existing pair. If it doesn't exist, it is created.
// This function is not thread safe.
func (cache *metricsCache) getOrCreate(name string) pair {
	pair, ok := cache.data[name]
	if !ok {
		pair = cache.newPair(name)
		cache.data[name] = pair
	}
	return pair
}

func (cache *metricsCache) newPair(name string) pair {
	return pair{
		meter:     metrics.NewRegisteredHistogram(name, cache.registry, newMovingWindowSample()),
		histogram: metrics.NewRegisteredHistogram(name, cache.registry, metrics.NewExpDecaySample(movingWindowSize, 0.015)),
	}
}
