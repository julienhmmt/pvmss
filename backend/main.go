package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	apiv1 "pvmss/api/v1"
	"pvmss/constants"
	"pvmss/database"
	envpkg "pvmss/env"
	"pvmss/handlers"
	"pvmss/logger"
	"pvmss/middleware"
	securityMiddleware "pvmss/security/middleware"
	"pvmss/state"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env before validation so env vars are available.
	if err := godotenv.Load("../.env"); err != nil {
		// Pre-init logger with defaults so we can log the warning.
		logger.Init(constants.DefaultLogLevel)
		logger.Get().Warn().Msg("No .env file found, using environment variables")
	}

	// Load and validate all required environment variables (fail-fast).
	envCfg, err := envpkg.LoadAndValidate()
	if err != nil {
		logger.Init(constants.DefaultLogLevel)
		logger.Get().Fatal().Err(err).Msg("Environment configuration invalid")
	}

	// Initialise logger with the validated log level from EnvConfig.
	logger.Init(envCfg.LogLevel)

	// Open (or create) the SQLite database before constructing the StateManager
	// so the DB handle can be injected at construction time.
	db, err := database.Open(envCfg.DBPath)
	if err != nil {
		logger.Get().Fatal().Err(err).Str("db_path", envCfg.DBPath).Msg("Failed to open database")
	}
	defer func() { _ = db.Close() }()

	stateManager := state.MakeAppStateWithDB(db)

	// Store EnvConfig on the state manager so all handlers can access it.
	stateManager.SetEnvConfig(envCfg)

	// Configure security middleware with the validated environment.
	securityMiddleware.SetProductionMode(envCfg.Environment)

	// Configure trusted proxies for X-Forwarded-For / X-Real-IP handling.
	// When PVMSS_TRUSTED_PROXIES is unset (empty), no proxies are trusted and
	// the middleware uses r.RemoteAddr directly, preventing IP spoofing.
	if envCfg.TrustedProxies != "" {
		cidrs := strings.Split(envCfg.TrustedProxies, ",")
		if err := middleware.SetTrustedProxies(cidrs); err != nil {
			logger.Get().Fatal().Err(err).Msg("Failed to parse PVMSS_TRUSTED_PROXIES")
		}
		logger.Get().Info().Str("trusted_proxies", envCfg.TrustedProxies).Msg("Trusted proxies configured")
	}

	logger.Get().Info().
		Str("event_category", "system").
		Str("event_type", "startup").
		Str("environment", envCfg.Environment).
		Bool("offline_mode", envCfg.Offline).
		Str("db_path", envCfg.DBPath).
		Str("log_level", envCfg.LogLevel).
		Msg("Starting PVMSS")

	logger.Get().Debug().Msg("Starting application initialization")
	if err := initializeApp(stateManager, db, envCfg); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize application")
	}
	logger.Get().Debug().Msg("Application initialization completed")

	port := envCfg.Port

	httpHandler, router := handlers.InitHandlers(stateManager)
	apiv1.RegisterRoutes(router, stateManager)
	apiv1.RegisterAdminDBRoutes(router, stateManager, db)
	apiv1.RegisterSetupRoutes(router, stateManager, db)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpHandler,
		ReadTimeout:       constants.ServerReadTimeout,
		WriteTimeout:      constants.ServerWriteTimeout,
		IdleTimeout:       constants.ServerIdleTimeout,
		ReadHeaderTimeout: constants.ServerReadHeaderTimeout,
		MaxHeaderBytes:    constants.MaxHeaderBytes,
	}

	go func() {
		logger.Get().Info().
			Str("event_category", "system").
			Str("event_type", "server_start").
			Str("port", port).
			Str("address", ":"+port).
			Msg("HTTP server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Get().Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Get().Info().
		Str("event_category", "system").
		Str("event_type", "shutdown_signal").
		Msg("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Get().Error().
			Str("event_category", "system").
			Str("event_type", "shutdown_error").
			Err(err).
			Msg("Server shutdown error")
	} else {
		logger.Get().Info().
			Str("event_category", "system").
			Str("event_type", "shutdown_complete").
			Msg("Server shutdown complete")
	}
}

func initializeApp(stateManager state.StateManager, db database.DB, envCfg *envpkg.EnvConfig) error {
	// T113: load settings from the database into the in-memory cache.
	if err := stateManager.LoadSettingsFromDB(); err != nil {
		return fmt.Errorf("load settings from database: %w", err)
	}

	if envCfg.Offline {
		logger.Get().Info().Msg("PVMSS_OFFLINE=true: starting in offline mode (Proxmox API calls disabled)")
		stateManager.SetOfflineMode()
	} else {
		if err := stateManager.StartOnlineMode(); err != nil {
			return fmt.Errorf("failed to start online mode: %w", err)
		}

		if connected, _ := stateManager.GetProxmoxStatus(); !connected {
			_, errorMsg := stateManager.GetProxmoxStatus()
			logger.Get().Warn().
				Str("error", errorMsg).
				Msg("Proxmox server not reachable, starting in read-only mode")
		} else {
			logger.Get().Info().Msg("Proxmox connection verified successfully")
		}
	}

	// Set frontend path in state manager
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("could not get current file path")
	}
	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")
	stateManager.SetFrontendPath(frontendPath)

	return nil
}
