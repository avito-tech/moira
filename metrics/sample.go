package metrics

import (
	"sync"

	"github.com/rcrowley/go-metrics"
)

const (
	movingWindowSize = 1028
)

// movingWindowSample is sample using moving window reservoir to keep selection of stream values.
type movingWindowSample struct {
	buffer      []int64
	count       int64
	index, size int
	overflow    bool
	lock        sync.RWMutex
}

func newMovingWindowSample() metrics.Sample {
	if metrics.UseNilMetrics {
		return metrics.NilSample{}
	}

	return &movingWindowSample{
		buffer: make([]int64, movingWindowSize),
		size:   movingWindowSize,
	}
}

// Clear clears all sample
func (mw *movingWindowSample) Clear() {
	mw.lock.Lock()
	defer mw.lock.Unlock()

	mw.count = 0
	mw.index = 0
	mw.overflow = false
}

// Count returns the number of samples recorded, which may exceed the reservoir size.
func (mw *movingWindowSample) Count() int64 {
	mw.lock.RLock()
	defer mw.lock.RUnlock()
	return mw.count
}

// Max returns the maximum value in the sample.
func (mw *movingWindowSample) Max() int64 {
	return metrics.SampleMax(mw.Values())
}

// Mean returns the mean of the values in the sample.
func (mw *movingWindowSample) Mean() float64 {
	return metrics.SampleMean(mw.Values())
}

// Min returns the minimum value in the sample
func (mw *movingWindowSample) Min() int64 {
	return metrics.SampleMin(mw.Values())
}

// Percentile returns an arbitrary percentile of values in the sample.
func (mw *movingWindowSample) Percentile(p float64) float64 {
	return metrics.SamplePercentile(mw.Values(), p)
}

// Percentiles returns a slice of arbitrary percentiles of values in the sample.
func (mw *movingWindowSample) Percentiles(ps []float64) []float64 {
	return metrics.SamplePercentiles(mw.Values(), ps)
}

// Size returns the size of the sample, which is at most the reservoir size.
func (mw *movingWindowSample) Size() int {
	mw.lock.RLock()
	defer mw.lock.RUnlock()

	if mw.overflow {
		return mw.size
	} else {
		return mw.index
	}
}

// Snapshot returns a read-only copy of the sample.
func (mw *movingWindowSample) Snapshot() metrics.Sample {
	mw.lock.RLock()
	defer mw.lock.RUnlock()

	return metrics.NewSampleSnapshot(mw.count, mw.Values())
}

// StdDev returns the standard deviation of the values in the sample.
func (mw *movingWindowSample) StdDev() float64 {
	return metrics.SampleStdDev(mw.Values())
}

// Sum returns the sum of the values in the sample.
func (mw *movingWindowSample) Sum() int64 {
	return metrics.SampleSum(mw.Values())
}

// Update samples a new value.
func (mw *movingWindowSample) Update(v int64) {
	mw.lock.Lock()
	defer mw.lock.Unlock()

	mw.buffer[mw.index] = v
	mw.count++
	mw.index++
	if mw.index == mw.size {
		mw.index = 0
		mw.overflow = true
	}
}

// Values returns a copy of the values in the sample.
func (mw *movingWindowSample) Values() []int64 {
	mw.lock.RLock()
	defer mw.lock.RUnlock()

	values := make([]int64, mw.size)
	if mw.overflow {
		// buffer has been overflow, so window's values consist of 2 parts:
		// from current position (included) to the end of buffer and from the beginning of buffer to current position (excluded)
		// e.g.: [7, 8, 3, 4, 5, 6], current position = 3, values are [3, 4, 5, 6, 7, 8]
		copied := copy(values, mw.buffer[mw.index:])
		if copied < mw.size {
			copy(values[copied:], mw.buffer[:mw.index])
		}
	} else {
		// buffer hasn't been overflown, so window's values is slice from beginning of buffer to current position (latter is excluded)
		// e.g.: [1, 2, 3, 4, 0, 0], current position = 4, values are [1, 2, 3, 4, 0, 0]
		copy(values, mw.buffer[0:mw.index])
	}
	return values
}

// Variance returns the variance of the values in the sample.
func (mw *movingWindowSample) Variance() float64 {
	return metrics.SampleVariance(mw.Values())
}
