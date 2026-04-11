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
	"pvmss/handlers"
	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"

	"github.com/joho/godotenv"
)

func main() {
	stateManager := state.MakeAppState()

	initLogger()

	// Log startup with environment context
	env := os.Getenv("PVMSS_ENV")
	if env == "" {
		env = "production"
	}
	offlineMode := strings.ToLower(os.Getenv("PVMSS_OFFLINE")) == "true"

	logger.Get().Info().
		Str("event_category", "system").
		Str("event_type", "startup").
		Str("environment", env).
		Bool("offline_mode", offlineMode).
		Msg("Starting PVMSS")

	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("No .env file found, using environment variables")
	}

	// Validate required environment variables for security
	if err := security.ValidateRequiredEnvVars(); err != nil {
		logger.Get().Fatal().Err(err).Msg("Environment validation failed - check your configuration")
	}

	logger.Get().Debug().Msg("Starting application initialization")
	if err := initializeApp(stateManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize application")
	}
	logger.Get().Debug().Msg("Application initialization completed")

	sessionManager, err := security.InitSecurity()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize security")
	}

	if err := stateManager.SetSessionManager(sessionManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to set session manager")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = constants.DefaultPort
	}

	httpHandler, router := handlers.InitHandlers(stateManager)
	apiv1.RegisterRoutes(router, stateManager)

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

func initLogger() {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = constants.DefaultLogLevel
	}
	logger.Init(level)
}

func initializeApp(stateManager state.StateManager) error {
	settings, modified, err := state.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Check if offline mode is enabled
	offlineMode := strings.ToLower(os.Getenv("PVMSS_OFFLINE")) == "true"
	if offlineMode {
		logger.Get().Info().Msg("Environment variable PVMSS_OFFLINE is set to true. Starting in offline mode (Proxmox API calls disabled)")
		stateManager.SetOfflineMode()
	} else {
		proxmoxURL := os.Getenv("PROXMOX_URL")
		tokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
		tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
		if proxmoxURL == "" || tokenID == "" || tokenValue == "" {
			return fmt.Errorf("missing required Proxmox environment variables: PROXMOX_URL, PROXMOX_API_TOKEN_NAME, PROXMOX_API_TOKEN_VALUE")
		}

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

	if modified {
		if err := stateManager.SetSettings(settings); err != nil {
			return fmt.Errorf("failed to save modified settings: %w", err)
		}
	} else {
		stateManager.SetSettingsWithoutSave(settings)
	}

	return nil
}
