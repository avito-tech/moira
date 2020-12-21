package metrics

type CheckerMetrics struct {
	CheckError  *Bucket
	CheckTime   *Bucket
	HandleError *Bucket
}

func NewCheckerMetrics() *CheckerMetrics {
	checkError, _ := NewBucket("checker.errors.check")
	checkTime, _ := NewBucket("checker.triggers")
	handleError, _ := NewBucket("checker.errors.handle")

	return &CheckerMetrics{
		CheckError:  checkError,
		CheckTime:   checkTime,
		HandleError: handleError,
	}
}
