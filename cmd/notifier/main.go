package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/cmd"
	"go.avito.ru/DO/moira/database/neo4j"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/fan"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/notifier"
	"go.avito.ru/DO/moira/notifier/escalations"
	"go.avito.ru/DO/moira/notifier/events"
	"go.avito.ru/DO/moira/notifier/notifications"
	"go.avito.ru/DO/moira/notifier/selfstate"
	"go.avito.ru/DO/moira/panicwrap"
	"go.avito.ru/DO/moira/sentry"
)

const (
	serviceName = "notifier"
)

var (
	logger                 moira.Logger
	configFileName         = flag.String("config", "/etc/moira/notifier.yml", "path to config file")
	printVersion           = flag.Bool("version", false, "Print current version and exit")
	printDefaultConfigFlag = flag.Bool("default-config", false, "Print default config and exit")
)

// Moira notifier bin version
var (
	MoiraVersion = "unknown"
	GitCommit    = "unknown"
	GoVersion    = "unknown"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()
	if *printVersion {
		fmt.Println("Moira Notifier")
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

	if err = metrics.Init(config.Statsd.GetSettings(), config.Notifier.LimitMetrics.GetSettings()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure metrics: %v\n", err)
		os.Exit(1)
	}

	if err = logging.Init(logging.ComponentNotifier, config.Rsyslog.GetSettings(), config.Notifier.LimitLogger.GetSettings()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure log: %v\n", err)
		os.Exit(1)
	}

	if err = sentry.Init(config.Notifier.Sentry.GetSettings(), serviceName); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure sentry: %v\n", err)
		os.Exit(1)
	}
	panicwrap.Init(serviceName)

	logger = logging.GetLogger("")
	defer logger.InfoF("Moira Notifier Stopped. Version: %s", MoiraVersion)

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
	notifierMetrics := metrics.NewNotifierMetrics()

	triggerInheritanceDatabase, err := neo4j.NewDatabase(logger, config.Neo4j)
	if err != nil {
		logger.FatalF("Can not configure Neo4j: %v", err)
	}

	notifierConfig := config.Notifier.getSettings(logger)
	sender := notifier.NewNotifier(database, notifierConfig, notifierMetrics)

	// Register moira senders
	if err := sender.RegisterSenders(database); err != nil {
		logger.FatalF("Can not configure senders: %v", err)
	}
	defer database.DeregisterBots()

	// Start moira self state checker
	selfState := &selfstate.SelfCheckWorker{
		Config:   config.Notifier.SelfState.getSettings(),
		DB:       database,
		Logger:   logger,
		Notifier: sender,
	}
	if err := selfState.Start(); err != nil {
		logger.FatalF("SelfState failed: %v", err)
	}
	defer stopSelfStateChecker(selfState)

	// Start moira notification fetcher
	fetchNotificationsWorker := &notifications.FetchNotificationsWorker{
		Logger:                     logger,
		Database:                   database,
		TriggerInheritanceDatabase: triggerInheritanceDatabase,
		Notifier:                   sender,
	}
	fetchNotificationsWorker.Start()
	defer stopNotificationsFetcher(fetchNotificationsWorker)

	// Start moira new events fetchers
	for _, withSaturations := range [...]bool{true, false} {
		// bring `withSaturations` into this scope
		// without this line, both workers would have `withSaturations == false`
		withSaturations := withSaturations

		fetchEventsWorker := &events.FetchEventsWorker{
			Logger:                     logger,
			Database:                   database,
			TriggerInheritanceDatabase: triggerInheritanceDatabase,
			Scheduler:                  notifier.NewScheduler(database, notifierMetrics),
			Metrics:                    notifierMetrics,
			Fan:                        fan.NewClient(config.Notifier.FanURL),

			Fetcher: func() (events moira.NotificationEvents, err error) {
				event, err := database.FetchNotificationEvent(withSaturations)
				return moira.NotificationEvents{event}, err
			},
		}
		fetchEventsWorker.Start()
		defer stopFetchEvents(fetchEventsWorker)
	}

	// Start moira delayed events fetchers
	for _, withSaturations := range [...]bool{true, false} {
		withSaturations := withSaturations
		fetchDelayedEventsWorker := &events.FetchEventsWorker{
			Logger:                     logger,
			Database:                   database,
			TriggerInheritanceDatabase: triggerInheritanceDatabase,
			Scheduler:                  notifier.NewScheduler(database, notifierMetrics),
			Metrics:                    notifierMetrics,
			Fan:                        fan.NewClient(config.Notifier.FanURL),

			Fetcher: func() (events moira.NotificationEvents, err error) {
				time.Sleep(5 * time.Second)
				return database.FetchDelayedNotificationEvents(time.Now().Unix(), withSaturations)
			},
		}
		fetchDelayedEventsWorker.Start()
		defer stopFetchEvents(fetchDelayedEventsWorker)
	}

	// Start moira escalations fetcher
	fetchEscalationsWorker := &escalations.FetchEscalationsWorker{
		Database: database,
		Notifier: sender,
	}
	fetchEscalationsWorker.Start()
	defer stopEscalationsFetcher(fetchEscalationsWorker)

	logger.InfoF("Moira Notifier Started. Version: %s", MoiraVersion)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info(fmt.Sprint(<-ch))
	logger.Info("Moira Notifier shutting down.")
}

func stopFetchEvents(worker *events.FetchEventsWorker) {
	if err := worker.Stop(); err != nil {
		logger.ErrorF("Failed to stop events fetcher: %v", err)
	}
}

func stopNotificationsFetcher(worker *notifications.FetchNotificationsWorker) {
	if err := worker.Stop(); err != nil {
		logger.ErrorF("Failed to stop notifications fetcher: %v", err)
	}
}

func stopSelfStateChecker(checker *selfstate.SelfCheckWorker) {
	if err := checker.Stop(); err != nil {
		logger.ErrorF("Failed to stop self check worker: %v", err)
	}
}

func stopEscalationsFetcher(worker *escalations.FetchEscalationsWorker) {
	if err := worker.Stop(); err != nil {
		logger.ErrorF("Failed to stop escalations fetcher: %v", err)
	}
}
