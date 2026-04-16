package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	apiv1 "pvmss/api/v1"
	"pvmss/constants"
	"pvmss/database"
	envpkg "pvmss/env"
	"pvmss/handlers"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
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

	// Also store in proxmox package for FromEnv convenience functions.
	proxmox.SetEnvConfig(envCfg)

	// Configure security middleware with the validated environment.
	securityMiddleware.SetProductionMode(envCfg.Environment)

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
	sessionManager, err := security.NewSessionManager(envCfg.SessionSecret, envCfg.Environment)
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	if err := stateManager.SetSessionManager(sessionManager); err != nil {
		return fmt.Errorf("failed to set session manager: %w", err)
	}

	// T114/T115/T116: handle bootstrap, migration, and first-run modes.
	if err := bootstrapSettings(db); err != nil {
		return fmt.Errorf("bootstrap settings: %w", err)
	}

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

// bootstrapSettings handles the three startup scenarios for settings persistence:
//
//   - T114  Bootstrap not complete + settings.json exists  → migrate JSON → DB
//   - T115  Bootstrap not complete + no settings.json     → first-run mode (use DB defaults)
//   - T116  Bootstrap complete     + settings.json exists  → log deprecation warning
func bootstrapSettings(db database.DB) error {
	log := logger.Get()

	bootstrapComplete, err := db.IsBootstrapComplete()
	if err != nil {
		return fmt.Errorf("check bootstrap status: %w", err)
	}

	settingsPath, pathErr := state.GetSettingsFilePath()
	settingsFileExists := pathErr == nil && fileExists(settingsPath)

	if bootstrapComplete {
		// T116: warn if settings.json is still present after a completed migration.
		if settingsFileExists {
			log.Warn().
				Str("settings_file", settingsPath).
				Msg("DEPRECATED: settings.json found but database bootstrap is already complete; settings.json is no longer read and can be removed")
		}
		return nil
	}

	if !settingsFileExists {
		// T115: no settings.json and bootstrap not done → first-run mode with DB defaults.
		log.Info().Msg("First-run mode: no settings.json found; starting with database defaults")
		if err := db.CompleteBootstrap(constants.AppVersion); err != nil {
			return fmt.Errorf("complete bootstrap (first-run): %w", err)
		}
		return nil
	}

	// T114: settings.json exists and bootstrap not complete → migrate.
	log.Info().Str("settings_file", settingsPath).Msg("Migrating settings from settings.json to database")

	// Create a backup of settings.json before migrating so the original
	// can be restored if the migration goes wrong.
	backupPath := settingsPath + ".pre-db-migration.bak"
	if err := backupFile(settingsPath, backupPath); err != nil {
		log.Warn().Err(err).Str("backup", backupPath).Msg("Failed to create settings.json backup; continuing migration")
	} else {
		log.Info().Str("backup", backupPath).Msg("Created settings.json backup before migration")
	}

	jsonSettings, err := database.ReadJSONSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("read settings.json for migration: %w", err)
	}
	summary, err := database.MigrateFromJSON(db, jsonSettings, "system")
	if err != nil {
		return fmt.Errorf("migrate settings.json to database: %w", err)
	}
	log.Info().
		Int("nodes", summary.NodesCount).
		Int("storages", summary.StoragesCount).
		Int("isos", summary.ISOsCount).
		Int("vmbrs", summary.VMBRsCount).
		Int("tags", summary.TagsCount).
		Int("cloudinit_templates", summary.CloudInitCount).
		Int("vm_profiles", summary.VMProfilesCount).
		Msg("Settings migrated from settings.json to database")
	return nil
}

// fileExists reports whether a regular file exists at path.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// backupFile copies src to dst. If dst already exists it is not overwritten.
func backupFile(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil // backup already exists, don't overwrite
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return out.Sync()
}
