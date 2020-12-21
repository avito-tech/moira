package metrics

type LoggerMetrics struct {
	Errors    *Bucket
	MsgSize   *Bucket
	MsgTotal  *Bucket
	Reconnect *Bucket
	Write     *Bucket
}

func NewLoggerMetric() *LoggerMetrics {
	errors, _ := NewBucket("rsyslog.errors.total")
	msgSize, _ := NewBucket("rsyslog.msg.size")
	msgTotal, _ := NewBucket("rsyslog.msg.total")
	reconnect, _ := NewBucket("rsyslog.reconnect.total")
	write, _ := NewBucket("rsyslog.time.write")

	return &LoggerMetrics{
		Errors:    errors,
		MsgSize:   msgSize,
		MsgTotal:  msgTotal,
		Reconnect: reconnect,
		Write:     write,
	}
}
