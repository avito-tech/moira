package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/cmd"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/filter"
	"go.avito.ru/DO/moira/filter/connection"
	"go.avito.ru/DO/moira/filter/heartbeat"
	"go.avito.ru/DO/moira/filter/matched_metrics"
	"go.avito.ru/DO/moira/filter/patterns"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/panicwrap"
	"go.avito.ru/DO/moira/sentry"
)

const (
	serviceName = "filter"
)

var (
	logger                 moira.Logger
	configFileName         = flag.String("config", "/etc/moira/filter.yml", "path config file")
	printVersion           = flag.Bool("version", false, "Print version and exit")
	printDefaultConfigFlag = flag.Bool("default-config", false, "Print default config and exit")
)

// Moira filter bin version
var (
	MoiraVersion = "unknown"
	GitCommit    = "unknown"
	GoVersion    = "unknown"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()
	if *printVersion {
		fmt.Println("Moira Filter")
		fmt.Println("Version:", MoiraVersion)
		fmt.Println("Git Commit:", GitCommit)
		fmt.Println("Go Version:", GoVersion)
		os.Exit(0)
	}

	config := getDefault()
	if *printDefaultConfigFlag {
		cmd.PrintConfig(config)
		os.Exit(0)
	}

	err := cmd.ReadConfig(*configFileName, &config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not read settings: %s\n", err.Error())
		os.Exit(1)
	}

	if err = metrics.Init(config.Statsd.GetSettings(), config.Filter.LimitMetrics.GetSettings()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure metrics: %v\n", err)
		os.Exit(1)
	}

	if err = logging.Init(logging.ComponentFilter, config.Rsyslog.GetSettings(), config.Filter.LimitLogger.GetSettings()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure log: %v\n", err)
		os.Exit(1)
	}

	if err = sentry.Init(config.Filter.Sentry.GetSettings(), serviceName); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure sentry: %v\n", err)
		os.Exit(1)
	}
	panicwrap.Init(serviceName)

	logger = logging.GetLogger("")
	defer logger.InfoF("Moira Filter stopped. Version: %s", MoiraVersion)

	if config.Pprof.Listen != "" {
		logger.InfoF("Starting pprof server at: [%s]", config.Pprof.Listen)
		cmd.StartProfiling(logger, config.Pprof)
	}

	if config.Liveness.Listen != "" {
		logger.InfoF("Starting liveness server at: [%s]", config.Liveness.Listen)
		cmd.StartLiveness(logger, config.Liveness)
	}

	if config.Filter.MaxParallelChecks == 0 {
		config.Filter.MaxParallelChecks = runtime.NumCPU()
	}

	cacheMetrics := metrics.NewFilterMetrics()
	database := redis.NewDatabase(logger, config.Redis.GetSettings())

	retentionConfigFile, err := os.Open(config.Filter.RetentionConfig)
	if err != nil {
		logger.FatalF("Error open retentions file [%s]: %v", config.Filter.RetentionConfig, err)
	}

	cacheStorage, err := filter.NewCacheStorage(retentionConfigFile)
	if err != nil {
		logger.FatalF("Failed to initialize cache storage with config [%s]: %v", config.Filter.RetentionConfig, err)
	}

	patternStorage, err := filter.NewPatternStorage(database, cacheMetrics, logger)
	if err != nil {
		logger.FatalF("Failed to refresh pattern storage: %s", err)
	}

	// Refresh Patterns on first init
	refreshPatternWorker := patterns.NewRefreshPatternWorker(database, logger, patternStorage)

	// Start patterns refresher
	err = refreshPatternWorker.Start()
	if err != nil {
		logger.FatalF("Failed to refresh pattern storage: %s", err.Error())
	}
	defer stopRefreshPatternWorker(refreshPatternWorker)

	// Start Filter heartbeat
	heartbeatWorker := heartbeat.NewHeartbeatWorker(database, logger, patternStorage.GetHeartbeat())
	heartbeatWorker.Start()
	defer stopHeartbeatWorker(heartbeatWorker)

	// Start metrics listener
	listener, err := connection.NewListener(config.Filter.Listen, logger)
	if err != nil {
		logger.FatalF("Failed to start listen: %s", err.Error())
		os.Exit(1)
	}
	lineChan := listener.Listen()

	matcherWorker := patterns.NewMatcherWorker(logger, patternStorage)
	metricsChan := matcherWorker.Start(config.Filter.MaxParallelChecks, lineChan)

	// Start metrics matcher
	metricsMatcher := matchedmetrics.NewMetricsMatcher(cacheMetrics, logger, database, cacheStorage)
	metricsMatcher.Start(metricsChan)
	defer metricsMatcher.Wait()  // First stop listener
	defer stopListener(listener) // Then waiting for metrics matcher handle all received events

	logger.InfoF("Moira Filter started. Version: %s", MoiraVersion)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info(fmt.Sprint(<-ch))
	logger.Info("Moira Filter shutting down.")
}

func stopListener(listener *connection.MetricsListener) {
	if err := listener.Stop(); err != nil {
		logger.ErrorF("Failed to stop listener: %v", err)
	}
}

func stopHeartbeatWorker(heartbeatWorker *heartbeat.Worker) {
	if err := heartbeatWorker.Stop(); err != nil {
		logger.ErrorF("Failed to stop heartbeat worker: %v", err)
	}
}

func stopRefreshPatternWorker(refreshPatternWorker *patterns.RefreshPatternWorker) {
	if err := refreshPatternWorker.Stop(); err != nil {
		logger.ErrorF("Failed to stop refresh pattern worker: %v", err)
	}
}
