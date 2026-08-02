package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
)

const (
	readHeaderTimeout = 2 * time.Second
	readTimeout       = 5 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20 // 1 MiB
)

func main() {
	os.Exit(run())
}

func run() int {
	stderr := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg, err := config.Load()
	if err != nil {
		stderr.Error("failed to load configuration", "component", "main", "error", err)
		return 1
	}

	logger, logCloser, err := config.NewLogger(cfg)
	if err != nil {
		stderr.Error("failed to create logger", "component", "main", "error", err)
		return 1
	}
	defer func() { _ = logCloser.Close() }()

	logger.Info("configuration loaded", "component", "main", "port", cfg.Port, "dbPath", cfg.DBPath)

	st, err := store.Open(cfg)
	if err != nil {
		logger.Error("failed to open database", "component", "main", "error", err)
		return 1
	}
	defer func() { _ = st.Close() }()
	logger.Info("database opened", "component", "main", "migrationsApplied", len(store.Migrations))

	webDir, err := resolveWebBuildDir(cfg.WebDir)
	if err != nil {
		logger.Error("web build directory not found", "component", "main", "error", err)
		return 1
	}
	logger.Info("web build directory resolved", "component", "main", "webDir", webDir)

	health := httpapi.NewHealth(st, logger)
	router := httpapi.NewRouter(health, webDir, logger)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	logger.Info("server listening", "component", "main", "addr", srv.Addr)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "component", "main", "error", err)
			return 1
		}
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "component", "main", "error", err)
			return 1
		}
	}

	logger.Info("server stopped", "component", "main")
	return 0
}

// resolveWebBuildDir returns the absolute path to the static web build directory.
// It checks PVMSS_WEB_DIR first, then falls back to a path relative to the
// executable, and finally to a path relative to the current working directory.
func resolveWebBuildDir(cfgWebDir string) (string, error) {
	if cfgWebDir != "" {
		return cfgWebDir, validateWebBuildDir(cfgWebDir)
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "..", "..", "web", "build")
		if validateWebBuildDir(candidate) == nil {
			return candidate, nil
		}
	}

	if validateWebBuildDir("web/build") == nil {
		return "web/build", nil
	}

	return "", fmt.Errorf("set PVMSS_WEB_DIR to a web build directory")
}

func validateWebBuildDir(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	return nil
}
