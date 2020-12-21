package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/patrickmn/go-cache"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/checker"
	"go.avito.ru/DO/moira/checker/worker"
	"go.avito.ru/DO/moira/cmd"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/panicwrap"
	"go.avito.ru/DO/moira/sentry"
	"go.avito.ru/DO/moira/silencer"
)

const (
	serviceName = "checker"
)

var (
	logger                 moira.Logger
	configFileName         = flag.String("config", "/etc/moira/checker.yml", "Path to configuration file")
	printVersion           = flag.Bool("version", false, "Print version and exit")
	printDefaultConfigFlag = flag.Bool("default-config", false, "Print default config and exit")
	triggerID              = flag.String("t", "", "Check single trigger by id and exit")
)

// Moira checker bin version
var (
	MoiraVersion = "unknown"
	GitCommit    = "unknown"
	GoVersion    = "unknown"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()
	if *printVersion {
		fmt.Println("Moira Checker")
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

	checkerSettings := config.Checker.getSettings()
	checkerSettings.Netbox = config.Netbox.GetSettings()

	if err = metrics.Init(config.Statsd.GetSettings(), checkerSettings.LimitMetrics); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure metrics: %v\n", err)
		os.Exit(1)
	}

	if err = logging.Init(logging.ComponentChecker, config.Rsyslog.GetSettings(), checkerSettings.LimitLogger); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure log: %v\n", err)
		os.Exit(1)
	}

	if err = sentry.Init(checkerSettings.Sentry, serviceName); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure sentry: %v\n", err)
		os.Exit(1)
	}
	panicwrap.Init(serviceName)

	logger = logging.GetLogger("")
	defer logger.InfoF("Moira Checker stopped. Version: %s", MoiraVersion)

	if config.Pprof.Listen != "" {
		logger.InfoF("Starting pprof server at: [%s]", config.Pprof.Listen)
		cmd.StartProfiling(logger, config.Pprof)
	}

	if config.Liveness.Listen != "" {
		logger.InfoF("Starting liveness server at: [%s]", config.Liveness.Listen)
		cmd.StartLiveness(logger, config.Liveness)
	}

	databaseSettings := config.Redis.GetSettings()
	database := redis.NewDatabase(logger, databaseSettings)

	checkerMetrics := metrics.NewCheckerMetrics()
	if triggerID != nil && *triggerID != "" {
		checkSingleTrigger(database, checkerMetrics, checkerSettings)
		return
	}

	logger.Debug("Starting silencer worker")
	s := silencer.NewSilencer(database, checkerSettings.Netbox)
	s.Start()
	defer s.Stop()

	checkerWorker := &worker.Checker{
		Logger:   logger,
		Database: database,
		Config:   checkerSettings,
		Metrics:  checkerMetrics,
		Cache:    cache.New(time.Minute, time.Minute*60),
	}

	err = checkerWorker.Start()
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer stopChecker(checkerWorker)

	logger.InfoF("Moira Checker started. Version: %s", MoiraVersion)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info(fmt.Sprint(<-ch))
	logger.Info("Moira Checker shutting down.")
}

func checkSingleTrigger(
	database moira.Database,
	metrics *metrics.CheckerMetrics,
	settings *checker.Config,
) {
	triggerChecker := checker.TriggerChecker{
		TriggerID: *triggerID,
		Database:  database,
		Config:    settings,
		Statsd:    metrics,
	}

	if err := triggerChecker.InitTriggerChecker(); err != nil {
		logger.FatalF("Failed initialize trigger checker: %s", err.Error())
	}

	if err := triggerChecker.Check(); err != nil {
		logger.FatalF("Failed check trigger: %s", err)
	}

	os.Exit(0)
}

func stopChecker(service *worker.Checker) {
	if err := service.Stop(); err != nil {
		logger.ErrorF("Failed to Stop Moira Checker: %v", err)
	}
}
