package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"qmi-web/internal/api"
	"qmi-web/internal/app"
	"qmi-web/internal/auth"
	"qmi-web/internal/config"
	"qmi-web/internal/database"
	"qmi-web/internal/device"
	"qmi-web/internal/logging"
	"qmi-web/internal/qmi"
	"qmi-web/internal/security"
	"qmi-web/internal/version"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Print(versionText())
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "reset-admin" {
		resetAdmin(os.Args[2:])
		return
	}
	runServer()
}

// versionText is deliberately backed by the same build metadata exposed at
// /version, so CLI, API and the WebUI cannot drift to separate version sources.
func versionText() string {
	build := version.Current()
	return fmt.Sprintf("qmi-web %s\ncommit %s\nbuild_time %s\n", build.Version, build.Commit, build.BuildTime)
}

func runServer() {
	var configPath, dataDir, logsDir string
	flag.StringVar(&configPath, "config", envOr("QMI_WEB_CONFIG", "/config/config.yaml"), "path to non-secret config")
	flag.StringVar(&dataDir, "data", envOr("QMI_WEB_DATA", "/data"), "persistent data directory")
	flag.StringVar(&logsDir, "logs", envOr("QMI_WEB_LOGS", "/logs"), "log directory")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fatal("load config", err)
	}
	if backend := os.Getenv("QMI_WEB_BACKEND"); backend != "" {
		cfg.Backend.Type = backend
	}
	if raw, ok := os.LookupEnv("QMI_WEB_SMS_ONLY"); ok {
		smsOnly, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			fatal("parse QMI_WEB_SMS_ONLY", err)
		}
		cfg.Security.SMSOnly = smsOnly
	}
	if controlPath, ok := os.LookupEnv("QMI_WEB_DEVICE"); ok {
		cfg.Device.ControlPath = strings.TrimSpace(controlPath)
	}
	if raw, ok := os.LookupEnv("QMI_WEB_REAL_VALIDATION"); ok {
		realValidation, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			fatal("parse QMI_WEB_REAL_VALIDATION", err)
		}
		cfg.Device.RealValidation = realValidation
	}
	if raw, ok := os.LookupEnv("QMI_WEB_REAL_VALIDATION_WINDOW"); ok {
		window, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			fatal("parse QMI_WEB_REAL_VALIDATION_WINDOW", err)
		}
		cfg.Device.RealValidationWindow = config.Duration(window)
	}
	if raw, ok := os.LookupEnv("QMI_WEB_REAL_VALIDATION_STATUS_FILE"); ok {
		cfg.Device.RealValidationStatusFile = strings.TrimSpace(raw)
	}
	if raw, ok := os.LookupEnv("QMI_WEB_SMS_RECONCILE_INTERVAL"); ok {
		interval, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			fatal("parse QMI_WEB_SMS_RECONCILE_INTERVAL", err)
		}
		cfg.SMS.ReconcileInterval = config.Duration(interval)
	}
	if raw, ok := os.LookupEnv("QMI_WEB_DEVICE_RECONNECT_MAX_BACKOFF"); ok {
		backoff, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			fatal("parse QMI_WEB_DEVICE_RECONNECT_MAX_BACKOFF", err)
		}
		cfg.Device.ReconnectMaxBackoff = config.Duration(backoff)
	}
	if err := cfg.Validate(); err != nil {
		fatal("validate runtime configuration", err)
	}
	store, err := database.Open(filepath.Join(dataDir, "qmi-web.db"))
	if err != nil {
		fatal("open database", err)
	}
	defer store.Close()
	cipher, err := security.LoadCipher(dataDir)
	if err != nil {
		fatal("load master key", err)
	}
	manager, err := device.NewManager(cfg.Backend.Type, qmi.NewBackend(cfg.Device.AllowedVIDs, cfg.Device.ControlPath), device.NewMockBackend())
	if err != nil {
		fatal("create device manager", err)
	}
	logger, err := logging.New(filepath.Join(logsDir, "qmi-web.jsonl"), cfg.Logging.Level, nil)
	if err != nil {
		fatal("open logger", err)
	}
	defer logger.Close()
	application := app.New(cfg, store, manager, cipher, logger)
	authentication := auth.New(store, dataDir, false)
	root, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	legacyTokenRemoved, err := authentication.Initialize(root)
	if err != nil {
		fatal("initialize local administrator", err)
	}
	if legacyTokenRemoved {
		logger.Info("legacy setup token removed", map[string]any{"event": "legacy_setup_token_removed"})
	}
	application.Start(root)
	server, err := api.New(application, authentication)
	if err != nil {
		fatal("create API", err)
	}
	httpServer := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.Info("QMI Web server started", map[string]any{"address": cfg.ListenAddress(), "backend": cfg.Backend.Type, "sms_only": true})
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", map[string]any{"error": err.Error()})
			cancel()
		}
	}()
	<-root.Done()
	shutdown, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	_ = httpServer.Shutdown(shutdown)
	_ = application.Shutdown()
	logger.Info("QMI Web server stopped", nil)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "qmi-web: %s: %v\n", action, err)
	os.Exit(1)
}
