package metrics

import (
	"fmt"
	"go.avito.ru/DO/moira"
	"os"
	"strings"
	"sync"
)

var (
	cfg    *Config
	mu     sync.Mutex
	worker *metricsWorker
)

func Init(config Config, rateLimit moira.RateLimit) error {
	mu.Lock()
	defer mu.Unlock()

	if cfg != nil {
		return fmt.Errorf("already initialized")
	}

	prefix, err := initPrefix(config.Prefix)
	if err != nil {
		return fmt.Errorf("Can not get OS hostname %s: %s", config.Prefix, err)
	}

	cfg = &config
	cfg.Limits = rateLimit
	cfg.Prefix = strings.TrimSuffix(prefix, ".")

	worker, err = newMetricsWorker()
	if err != nil {
		return fmt.Errorf("Can not initialize statsd: %v", err)
	}

	go worker.lifeCycle()
	return nil
}

func initPrefix(prefix string) (string, error) {
	const hostnameTmpl = "{hostname}"
	if !strings.Contains(prefix, hostnameTmpl) {
		return prefix, nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return prefix, err
	}

	short := strings.Split(hostname, ".")[0]
	return strings.ReplaceAll(prefix, hostnameTmpl, short), nil
}
