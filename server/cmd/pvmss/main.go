// Command pvmss is the next-gen PVMSS server entry point: it wires the
// config, store, cluster client, inventory projection and HTTP API together
// and serves the SPA + REST endpoints on a single port.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strconv"
	"syscall"
	"time"
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

	logger.Info("configuration loaded", "component", "main", "host", cfg.Host, "port", cfg.Port, "dbPath", cfg.DBPath)

	st, err := store.Open(cfg)
	if err != nil {
		logger.Error("failed to open database", "component", "main", "error", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	logger.Info("database opened", "component", "main", "migrationsDefined", len(store.Migrations))

	webDir, err := resolveWebBuildDir(cfg.WebDir)
	if err != nil {
		logger.Error("web build directory not found", "component", "main", "error", err)
		return 1
	}

	logger.Info("web build directory resolved", "component", "main", "webDir", webDir)

	// This is the ONLY site that selects between cluster.Client implementations
	// (SC-004) — no other package may branch on cfg.ClusterSource.
	var clusterClient cluster.Client

	switch cfg.ClusterSource {
	case "proxmox":
		clusterClient = cluster.Proxmox{}
	default:
		clusterClient = cluster.Fake{}
	}

	logger.Info("cluster client selected", "component", "cluster", "source", cfg.ClusterSource, "cluster", "default")

	// The inventory projection owns all reads of cluster data (FR-002, SC-004).
	// The worker refreshes it periodically; the refresher handles manual
	// refresh requests with a minimum-interval guard (FR-005, FR-006).
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(
		clusterClient,
		projection,
		cfg.InventoryRefreshInterval,
		logger,
		inventory.WithRefreshTimeout(cfg.InventoryRefreshTimeout),
	)
	refresher := inventory.NewRefresher(worker, cfg.InventoryManualRefreshMinInterval)

	// Start the worker before the HTTP server accepts traffic (T015) so the
	// projection is populated before the first request can arrive.
	inventoryCtx, cancelInventory := context.WithCancel(context.Background())
	defer cancelInventory()

	go worker.Run(inventoryCtx)

	sessions, err := auth.NewSessionManager(st, cfg.SessionSecret, false)
	if err != nil {
		logger.Error("failed to create session manager", "component", "main", "error", err)
		return 1
	}

	health := httpapi.NewHealth(st, logger)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(refresher, logger)
	authHandler := httpapi.NewAuth(clusterClient, sessions, cfg.AdminPasswordHash, auth.NewTokenService(st), logger)
	vms := httpapi.NewVMs(projection, authHandler, cfg.MaxListPageSize, cfg.DefaultUserQuota, logger)
	// Both cluster.Client implementations (Fake, Proxmox) also implement
	// cluster.Writer — reads and writes are separated by interface
	// (constitution IV), not by implementation. The assertion is safe because
	// the switch above only selects values that satisfy both.
	writer, ok := clusterClient.(cluster.Writer)
	if !ok {
		logger.Error("cluster client does not implement Writer", "component", "main")
		return 1
	}

	creator, ok := clusterClient.(cluster.Creator)
	if !ok {
		logger.Error("cluster client does not implement Creator", "component", "main")
		return 1
	}

	vmDetail := httpapi.NewVMDetail(projection, authHandler, writer, st, worker, logger)
	vmCreate := httpapi.NewVMCreate(authHandler, st, creator, logger)
	tasks := httpapi.NewTasks(authHandler, creator, worker, logger)
	router := httpapi.NewRouter(health, clusterNodes, clusterRefresh, vms, vmDetail, vmCreate, tasks, authHandler, webDir, logger)

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
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

	sigCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
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

	return "", errors.New("set PVMSS_WEB_DIR to a web build directory")
}

func validateWebBuildDir(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}

	if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}

	indexPath := filepath.Join(path, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return fmt.Errorf("%q is missing index.html: %w", path, err)
	}

	return nil
}
