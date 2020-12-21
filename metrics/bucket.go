package metrics

import (
	"fmt"
	"time"
)

// Bucket holds metrics bucket (name)
type Bucket struct {
	prefix string
}

func NewBucket(prefix string) (*Bucket, error) {
	if cfg == nil {
		return nil, fmt.Errorf("not initialized yet")
	}

	return &Bucket{prefix: prefix}, nil
}

// Count adds n to bucket.
func (b *Bucket) Count(n int) {
	worker.addCall(&delayedCall{
		callType: ctCount,
		bucket:   b.prefix,
		value:    int64(n),
	})
}

// GetCount returns current value of counter
func (b *Bucket) GetCount() int64 {
	return worker.getCount(b.prefix)
}

// Histogram sends an histogram value to a bucket.
func (b *Bucket) Histogram(value int64) {
	worker.addCall(&delayedCall{
		callType: ctHistogram,
		bucket:   b.prefix,
		value:    value,
	})
}

// Increment is equivalent for Count(1)
func (b *Bucket) Increment() {
	worker.addCall(&delayedCall{
		callType: ctCount,
		bucket:   b.prefix,
		value:    1,
	})
}

// Timing sends a timing value to a bucket.
func (b *Bucket) Timing(value int64) {
	worker.addCall(&delayedCall{
		callType: ctTiming,
		bucket:   b.prefix,
		value:    value,
	})
}

// UpdateSince sends a timing value (nanoseconds) that passed since the given time
func (b *Bucket) UpdateSince(t time.Time) {
	worker.addCall(&delayedCall{
		callType: ctTiming,
		bucket:   b.prefix,
		value:    int64(time.Since(t)),
	})
}

// Flush flushes current calls buffer
func (b *Bucket) Flush() {
	worker.flush()
}
