package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"process-guardian/internal/config"
	"process-guardian/internal/guardian"
	"process-guardian/pkg/logger"
)

var (
	version   = "1.0.0"
	buildTime = "2026-08-13T00:00:00Z"
)

func main() {
	showVersion := flag.Bool("version", false, "Print version information and exit")
	showHelp := flag.Bool("help", false, "Show help message and exit")
	configFile := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	logger.Info("Process Guardian Service v%s starting", version)
	logger.Infof("Build time: %s, Go version: %s, Platform: %s/%s",
		buildTime, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	cfg := config.LoadFromEnv()

	if *configFile != "" {
		fileCfg, err := config.LoadFromFile(*configFile)
		if err != nil {
			logger.Warn("Failed to load config file %s, using env defaults: %v", *configFile, err)
		} else {
			cfg = fileCfg
		}
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid configuration: %v", err)
	}

	logger.Infof("Configuration: host=%s port=%d health_interval=%v restart_delay=%v max_restarts=%d shutdown_timeout=%v",
		cfg.Host, cfg.Port, cfg.HealthInterval, cfg.RestartDelay, cfg.MaxRestartCount, cfg.ShutdownTimeout)

	logLevel := parseLogLevel(cfg.LogLevel)
	logger.SetLevel(logLevel)

	logger.Infof("Log level set to: %s", cfg.LogLevel)

	mgr := guardian.NewManager(cfg)
	mgr.Start()

	router := NewRouter(mgr)
	router.SetupRoutes()

	srv := &http.Server{
		Addr:         cfg.Address(),
		Handler:      router.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Infof("HTTP server listening on %s", cfg.Address())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Infof("Received signal %v, initiating graceful shutdown...", sig)
	case err := <-serverErrors:
		logger.Errorf("Server error, initiating shutdown: %v", err)
	}

	logger.Info("Stopping HTTP server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("HTTP server shutdown error: %v", err)
	}

	logger.Info("Stopping process manager...")
	mgr.Shutdown()

	logger.Info("Process Guardian Service stopped")
}

func printVersion() {
	fmt.Printf("Process Guardian v%s\n", version)
	fmt.Printf("Build time: %s\n", buildTime)
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println("Process Guardian - Lightweight Process Guardian and Health Monitoring Service")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  process-guardian [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -config string   Path to configuration file")
	fmt.Println("  -version         Print version information and exit")
	fmt.Println("  -help            Show this help message and exit")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  GUARDIAN_PORT              Server port (default: 8080)")
	fmt.Println("  GUARDIAN_HOST              Server host (default: 0.0.0.0)")
	fmt.Println("  GUARDIAN_HEALTH_INTERVAL   Health check interval (default: 10s)")
	fmt.Println("  GUARDIAN_RESTART_DELAY     Restart delay (default: 2s)")
	fmt.Println("  GUARDIAN_MAX_RESTART       Max restart count (default: 5)")
	fmt.Println("  GUARDIAN_SHUTDOWN_TIMEOUT  Shutdown timeout (default: 5s)")
	fmt.Println("  GUARDIAN_LOG_LEVEL         Log level (default: info)")
	fmt.Println("  GUARDIAN_CONFIG            Path to config file")
	fmt.Println()
	fmt.Println("API Endpoints:")
	fmt.Println("  POST /process                Register and start a new process")
	fmt.Println("  GET  /processes              List all processes")
	fmt.Println("  GET  /process/{name}         Get process details")
	fmt.Println("  POST /process/{name}/stop    Stop a process")
	fmt.Println("  POST /process/{name}/start   Start a stopped process")
	fmt.Println("  DELETE /process/{name}       Remove a process")
	fmt.Println("  GET  /health                 Health check")
	fmt.Println("  GET  /status                 Service status")
}

func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.LevelDebug
	case "info":
		return logger.LevelInfo
	case "warn":
		return logger.LevelWarn
	case "error":
		return logger.LevelError
	case "fatal":
		return logger.LevelFatal
	default:
		return logger.LevelInfo
	}
}

func init() {
	logger.Default()
}