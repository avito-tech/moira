package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api/handler"
	"go.avito.ru/DO/moira/cmd"
	"go.avito.ru/DO/moira/database/neo4j"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/metrics"
	"go.avito.ru/DO/moira/panicwrap"
	"go.avito.ru/DO/moira/sentry"
)

const (
	serviceName = "api"
)

var (
	logger                 moira.Logger
	configFileName         = flag.String("config", "/etc/moira/api.yml", "Path to configuration file")
	printVersion           = flag.Bool("version", false, "Print version and exit")
	printDefaultConfigFlag = flag.Bool("default-config", false, "Print default config and exit")
)

// Moira api bin version
var (
	MoiraVersion = "unknown"
	GitCommit    = "unknown"
	GoVersion    = "unknown"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()
	if *printVersion {
		fmt.Println("Moira Api")
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

	apiConfig := config.API.getSettings()
	apiConfig.Netbox = config.Netbox.GetSettings()

	if err = metrics.Init(config.Statsd.GetSettings(), apiConfig.LimitMetrics); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure metrics: %v\n", err)
		os.Exit(1)
	}

	if err = logging.Init(logging.ComponentApi, config.Rsyslog.GetSettings(), apiConfig.LimitLogger); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure log: %v\n", err)
		os.Exit(1)
	}

	if err = sentry.Init(apiConfig.Sentry, serviceName); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Can not configure sentry: %v\n", err)
		os.Exit(1)
	}
	panicwrap.Init(serviceName)

	logger = logging.GetLogger("")
	defer logger.InfoF("Moira API Stopped. Version: %s", MoiraVersion)

	configFileContent, err := moira.GetFileContent(config.API.WebConfigPath)
	if err != nil {
		logger.WarnF("Failed to read web config file by path '%s', method 'api/config' will be return 404, error: %s'", config.API.WebConfigPath, err.Error())
	}

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

	neo4jDb, err := neo4j.NewDatabase(logger, config.Neo4j)
	if err != nil {
		logger.FatalF("Can not configure Neo4j: %s\n", err.Error())
	}

	listener, err := net.Listen("tcp", apiConfig.Listen)
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.InfoF("Start listening by address: [%s]", apiConfig.Listen)

	httpHandler := handler.NewHandler(database, neo4jDb, logger, apiConfig, []byte(configFileContent), MoiraVersion)
	server := &http.Server{
		Handler: httpHandler,
	}

	go func() {
		_ = server.Serve(listener)
	}()
	defer Stop(logger, server)

	logger.InfoF("Moira Api Started (version: %s)", MoiraVersion)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info(fmt.Sprint(<-ch))
	logger.Info("Moira API shutting down.")
}

// Stop Moira API HTTP server
func Stop(logger moira.Logger, server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.ErrorF("Can't stop Moira API correctly: %v", err)
	}
	logger.InfoF("Moira API Stopped. Version: %s", MoiraVersion)
}
