package patterns

import (
	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/filter"
)

type Worker struct {
	logger         moira.Logger
	tomb           tomb.Tomb
	patternStorage *filter.PatternStorage
}

// NewHeartbeatWorker creates new worker
func NewMatcherWorker(logger moira.Logger, patternsStorage *filter.PatternStorage) *Worker {
	return &Worker{
		logger:         logger,
		patternStorage: patternsStorage,
	}
}

func (w *Worker) Start(workerQty int, lineChan <-chan []byte) chan *moira.MatchedMetric {
	metricsChan := make(chan *moira.MatchedMetric, 10000)
	w.logger.InfoF("starting %d matcher workers", workerQty)

	for i := 0; i < workerQty; i++ {
		w.tomb.Go(func() error {
			return w.worker(lineChan, metricsChan)
		})
	}

	go func() {
		for {
			select {
			case <-w.tomb.Dying():
				{
					w.logger.Info("Stopping matcher...")
					close(metricsChan)
					w.logger.Info("Moira Filter matcher stopped")
					return
				}
			}
		}
	}()

	return metricsChan
}

func (w *Worker) worker(in <-chan []byte, out chan<- *moira.MatchedMetric) error {
	for line := range in {
		if m := w.patternStorage.ProcessIncomingMetric(line); m != nil {
			out <- m
		}
	}
	return nil
}
