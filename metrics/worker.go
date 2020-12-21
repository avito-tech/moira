package metrics

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/alexcesaro/statsd.v2"
	"gopkg.in/tomb.v2"
)

const (
	bufferSize      = 1048576
	bufferThreshold = 786432
	connectionsQty  = 16
	flushInterval   = 10 * time.Second
)

const (
	ctCount callType = iota
	ctHistogram
	ctTiming
)

type callType int

type delayedCall struct {
	callType callType
	bucket   string
	value    int64
}

type metricsWorker struct {
	caches  []*metricsCache
	clients []*statsd.Client
	locks   []sync.Mutex
	queue   chan delayedCall
	rate    float64
	threads int
	tomb    tomb.Tomb

	// simple map-based counter for test purposes only
	counter map[string]int64
	sync    chan bool
}

type sample struct {
	name   string
	countM int64
	countH int64
	values []int64
}

func newMetricsWorker() (*metricsWorker, error) {
	var (
		err     error
		threads int
		options []statsd.Option
	)

	options = []statsd.Option{
		statsd.Address(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		statsd.FlushPeriod(1 * time.Second),
		statsd.Prefix(cfg.Prefix),
		statsd.Network("udp"),
		statsd.Mute(!cfg.Enabled),
	}
	threads = cfg.Limits.ThreadsQty

	caches := make([]*metricsCache, threads)
	for i := 0; i < threads; i++ {
		caches[i] = newMetricsCache()
	}

	clients := make([]*statsd.Client, connectionsQty)
	for i := 0; i < connectionsQty; i++ {
		clients[i], err = statsd.New(options...)
		if err != nil {
			return nil, err
		}
	}

	result := &metricsWorker{
		caches:  caches,
		clients: clients,
		locks:   make([]sync.Mutex, threads),
		queue:   make(chan delayedCall, bufferSize),
		rate:    cfg.Limits.AcceptRate,
		threads: threads,
		counter: make(map[string]int64),
		sync:    make(chan bool, 1),
	}
	return result, nil
}

func (worker *metricsWorker) addCall(delayedCall *delayedCall) {
	worker.queue <- *delayedCall

	if cfg.IsTest {
		<-worker.sync
	}
}

func (worker *metricsWorker) consumer(id int) error {
	var (
		call      delayedCall
		cache     *metricsCache
		pair      pair
		dropped   int64
		queueSize int
		skip      bool
	)

	for {
		select {
		case <-worker.tomb.Dying():
			return nil
		case call = <-worker.queue:
			// pass
		}

		skip = !cfg.Enabled || (worker.rate != 1 && worker.rate < rand.Float64())
		if skip {
			if cfg.IsTest {
				worker.counter[call.bucket] += call.value
				worker.sync <- true
			}
			continue
		}

		// drop metrics if queue is about to overflow
		queueSize = len(worker.queue)
		if queueSize > bufferThreshold {
			dropped++
			continue
		}

		worker.locks[id].Lock()
		cache = worker.caches[id]

		// record self metrics
		cache.queueSize.meter.Update(int64(queueSize))
		if dropped > 0 {
			cache.droppedCalls.meter.Update(dropped)
		}
		dropped = 0

		// record requested metric
		pair = cache.getOrCreate(call.bucket)
		switch call.callType {
		case ctCount:
			pair.meter.Update(call.value)
		case ctHistogram:
		case ctTiming:
			pair.histogram.Update(call.value)
		}
		worker.locks[id].Unlock()
	}
}

func (worker *metricsWorker) flush() {
	started := time.Now()

	// swap current metric caches with new ones (which are empty)
	caches := make([]*metricsCache, worker.threads)
	for i := 0; i < worker.threads; i++ {
		caches[i] = newMetricsCache()
		worker.locks[i].Lock()
		caches[i], worker.caches[i] = worker.caches[i], caches[i]
		worker.locks[i].Unlock()
	}

	// aggregate metric caches
	total := make(map[string]*sample)
	for i := 0; i < worker.threads; i++ {
		for name, pair := range caches[i].data {
			meterCount := pair.meter.Count()
			histogramCount := pair.histogram.Count()
			if meterCount == 0 && histogramCount == 0 {
				continue
			}
			histogramValues := pair.histogram.Sample().Values()

			data, ok := total[name]
			if !ok {
				data = &sample{name: name}
				total[name] = data
			}

			data.countM += meterCount
			data.countH += histogramCount
			data.values = append(data.values, histogramValues...)
		}
	}

	// emit aggregated data
	source := make(chan *delayedCall, connectionsQty)
	wg := sync.WaitGroup{}
	wg.Add(connectionsQty)
	for i := 0; i < connectionsQty; i++ {
		go func(id int, wg *sync.WaitGroup) {
			defer wg.Done()

			client := worker.clients[id]
			for data := range source {
				switch data.callType {
				case ctCount:
					client.Count(data.bucket, data.value)
				case ctHistogram:
					client.Timing(data.bucket, data.value)
				}
			}
		}(i, &wg)
	}

	for bucket, data := range total {
		if data.countM > 0 { // counter is emitted as is
			source <- &delayedCall{
				callType: ctCount,
				bucket:   bucket,
				value:    data.countM,
			}
		}
		if data.countH > 0 { //
			for _, value := range data.values { // each value of histogram's sample is emitted separately
				source <- &delayedCall{
					callType: ctHistogram,
					bucket:   bucket,
					value:    value,
				}
			}

			// histogram keeps only limited buffer of events, but its count is real
			// so if the buffer has been truncated, it is needed to emit increasing `.count` suffix
			if data.countH != int64(len(data.values)) {
				source <- &delayedCall{
					callType: ctCount,
					bucket:   bucket + ".count",
					value:    data.countH,
				}
			}
		}
	}
	close(source)
	wg.Wait()

	worker.clients[0].Timing(sdFlushTime, int64(time.Since(started)))
	worker.counter = make(map[string]int64)
}

func (worker *metricsWorker) flushTicker() error {
	if cfg.IsTest { // there is no periodical flushing in testing mode
		return nil
	}

	ticker := time.NewTicker(flushInterval)
	for {
		select {
		case <-worker.tomb.Dying():
			return nil
		case <-ticker.C:
			worker.flush()
		}
	}
}

func (worker *metricsWorker) getCount(bucket string) int64 {
	return worker.counter[bucket]
}

func (worker *metricsWorker) lifeCycle() {
	worker.start()
	defer worker.stop()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func (worker *metricsWorker) start() {
	for i := 0; i < worker.threads; i++ {
		func(id int) {
			worker.tomb.Go(func() error {
				return worker.consumer(id)
			})
		}(i)
	}
	worker.tomb.Go(worker.flushTicker)
}

func (worker *metricsWorker) stop() {
	worker.tomb.Kill(nil)
	worker.flush()
	_ = worker.tomb.Wait()
}
