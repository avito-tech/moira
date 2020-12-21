package metrics

type NotifierMetrics struct {
	SubsMalformed          *Bucket
	EventsReceived         *Bucket
	EventsMalformed        *Bucket
	EventsProcessingFailed *Bucket
	SendingFailed          *Bucket
	SendersOkMetrics       *Map
	SendersFailedMetrics   *Map
}

func NewNotifierMetrics() *NotifierMetrics {
	subsMalformed, _ := NewBucket("notifier.subs.malformed")
	eventsReceived, _ := NewBucket("notifier.events.received")
	eventsMalformed, _ := NewBucket("notifier.events.malformed")
	eventsProcessingFailed, _ := NewBucket("notifier.events.failed")
	sendingFailed, _ := NewBucket("notifier.sending.failed")
	senderOkMetrics := newMap()
	senderFailedMetrics := newMap()

	return &NotifierMetrics{
		SubsMalformed:          subsMalformed,
		EventsReceived:         eventsReceived,
		EventsMalformed:        eventsMalformed,
		EventsProcessingFailed: eventsProcessingFailed,
		SendingFailed:          sendingFailed,
		SendersOkMetrics:       senderOkMetrics,
		SendersFailedMetrics:   senderFailedMetrics,
	}
}
