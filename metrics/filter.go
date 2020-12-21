package metrics

type FilterMetrics struct {
	TotalMetricsReceived    *Bucket
	ValidMetricsReceived    *Bucket
	MatchingMetricsReceived *Bucket
	MatchingTimer           *Bucket
	SavingTimer             *Bucket
	BuildTreeTimer          *Bucket
}

func NewFilterMetrics() *FilterMetrics {
	totalMetricsReceived, _ := NewBucket("filter.received.total")
	validMetricsReceived, _ := NewBucket("filter.received.valid")
	matchingMetricsReceived, _ := NewBucket("filter.received.matching")
	matchingTimer, _ := NewBucket("filter.time.match")
	savingTimer, _ := NewBucket("filter.time.save")
	buildTreeTimer, _ := NewBucket("filter.time.buildtree")

	return &FilterMetrics{
		TotalMetricsReceived:    totalMetricsReceived,
		ValidMetricsReceived:    validMetricsReceived,
		MatchingMetricsReceived: matchingMetricsReceived,
		MatchingTimer:           matchingTimer,
		SavingTimer:             savingTimer,
		BuildTreeTimer:          buildTreeTimer,
	}
}
