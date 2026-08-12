// Command pvmss is the next-gen PVMSS server entry point: it wires the
// config, store, cluster client, inventory projection and HTTP API together
// and serves the SPA + REST endpoints on a single port.
//
//nolint:wsl_v5 // composition root keeps dependency wiring readable
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
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
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

	// appVersion is the version string surfaced in the dashboard, the admin
	// app info page, and the public /api/v1/public/version endpoint (T14).
	// It is a compile-time literal; no runtime discovery is performed.
	appVersion = "0.4.0-dev"
)

func main() {
	os.Exit(run())
}

//nolint:gocyclo,funlen // the composition root intentionally wires all runtime dependencies
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

	// Cluster implementation selection remains centralized in the registry wiring;
	// runtime rows change the number of clients, never the selected source kind.
	rows, err := st.ListClusters(context.Background())
	if err != nil {
		logger.Error("failed to list configured clusters", "component", "main", "error", err)
		return 1
	}
	clusterRegistry, err := cluster.NewRegistry(cfg.ClusterSource, rows)
	if err != nil {
		logger.Error("failed to create cluster registry", "component", "main", "error", err)
		return 1
	}
	clusterClient, err := clusterRegistry.Client("default")
	if err != nil {
		logger.Error("default cluster is unavailable", "component", "main", "error", err)
		return 1
	}
	logger.Info("cluster registry initialized", "component", "cluster", "source", cfg.ClusterSource, "clusters", clusterRegistry.List())

	// The inventory registry owns all reads of cluster data. Each active cluster
	// has an independent projection and refresh worker (FR-002, AC03 §2.4).
	inventoryRegistry := inventory.NewRegistry(
		clusterRegistry,
		cfg.InventoryRefreshInterval,
		logger,
		inventory.WithRefreshTimeout(cfg.InventoryRefreshTimeout),
	)
	inventoryRegistry.SetManualRefreshMinInterval(cfg.InventoryManualRefreshMinInterval)
	defaultProjection, err := inventoryRegistry.Projection("default")
	if err != nil {
		logger.Error("default inventory projection is unavailable", "component", "main", "error", err)
		return 1
	}
	defaultWorker, err := inventoryRegistry.Worker("default")
	if err != nil {
		logger.Error("default inventory worker is unavailable", "component", "main", "error", err)
		return 1
	}
	defaultRefresher, err := inventoryRegistry.Refresher("default")
	if err != nil {
		logger.Error("default inventory refresher is unavailable", "component", "main", "error", err)
		return 1
	}

	// Start every worker before the HTTP server accepts traffic (T015) so the
	// projections are populated before the first request can arrive.
	inventoryCtx, cancelInventory := context.WithCancel(context.Background())
	defer cancelInventory()
	inventoryRegistry.Start(inventoryCtx)

	sessions, err := auth.NewSessionManager(st, cfg.SessionSecret, cfg.CookieSecure)
	if err != nil {
		logger.Error("failed to create session manager", "component", "main", "error", err)
		return 1
	}

	router, err := buildRouter(cfg, clusterRegistry, inventoryRegistry, clusterClient, defaultProjection, defaultRefresher, defaultWorker, sessions, st, webDir, logger)
	if err != nil {
		logger.Error("failed to build router", "component", "main", "error", err)
		return 1
	}

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

// buildRouter wires all HTTP handlers into the final router. It performs the
// cluster.Writer/Creator type assertions (both Fake and Proxmox satisfy them)
// and constructs the handler graph from the shared dependencies.
func buildRouter(
	cfg config.Configuration,
	clusterRegistry *cluster.Registry,
	inventoryRegistry *inventory.Registry,
	clusterClient cluster.Client,
	projection *inventory.Projection,
	refresher *inventory.Refresher,
	worker *inventory.Worker,
	sessions *auth.SessionManager,
	st *store.Store,
	webDir string,
	logger *slog.Logger,
) (http.Handler, error) {
	policyService := policy.New(st, projection, clusterClient)
	health := httpapi.NewHealth(st, logger)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(refresher, logger)
	authHandler := httpapi.NewAuthWithRegistry(clusterRegistry, st, sessions, cfg.AdminPasswordHash, auth.NewTokenService(st), logger)
	vms := httpapi.NewVMsWithRegistry(inventoryRegistry, authHandler, cfg.MaxListPageSize, 0, logger, policyService)

	// Both cluster.Client implementations (Fake, Proxmox) also implement
	// cluster.Writer — reads and writes are separated by interface
	// (constitution IV), not by implementation. The assertion is safe because
	// the switch above only selects values that satisfy both.
	writer, ok := clusterClient.(cluster.Writer)
	if !ok {
		return nil, errors.New("cluster client does not implement Writer")
	}

	creator, ok := clusterClient.(cluster.Creator)
	if !ok {
		return nil, errors.New("cluster client does not implement Creator")
	}

	cloudInitReader, ok := clusterClient.(cluster.CloudInitReader)
	if !ok {
		return nil, errors.New("cluster client does not implement CloudInitReader")
	}

	snapshotReader, ok := clusterClient.(cluster.SnapshotReader)
	if !ok {
		return nil, errors.New("cluster client does not implement SnapshotReader")
	}

	snapshotWriter, ok := clusterClient.(cluster.SnapshotWriter)
	if !ok {
		return nil, errors.New("cluster client does not implement SnapshotWriter")
	}

	consoleRelay, ok := clusterClient.(cluster.ConsoleRelay)
	if !ok {
		return nil, errors.New("cluster client does not implement ConsoleRelay")
	}

	consoleTickets := vm.NewConsoleTicketStore()

	vmDetail := httpapi.NewVMDetailWithRegistry(inventoryRegistry, projection, authHandler, writer, st, worker, logger, policyService)
	vmCloudInit := httpapi.NewVMCloudInit(projection, authHandler, cloudInitReader, writer, st, worker, logger, policyService)
	vmCreate := httpapi.NewVMCreate(authHandler, st, creator, logger, policyService)
	tasks := httpapi.NewTasks(authHandler, creator, worker, logger)
	snapshots := httpapi.NewVMSnapshots(projection, authHandler, snapshotReader, snapshotWriter, st, logger, policyService)
	vmConsole := httpapi.NewVMConsole(projection, authHandler, consoleRelay, consoleTickets, st, logger)
	adminCatalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, clusterRegistry, projection, logger)
	adminPolicy := httpapi.NewAdminPolicyWithRegistry(authHandler, policyService, clusterRegistry, logger)
	adminPools := httpapi.NewAdminPools(authHandler, clusterClient, projection, writer, st, worker, logger)
	adminOps := httpapi.NewAdminOps(authHandler, st, clusterClient, projection, appVersion, logger)
	adminClusters := httpapi.NewAdminClusters(authHandler, st, clusterRegistry, inventoryRegistry, logger)

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health:           health,
		ClusterNodes:     clusterNodes,
		ClusterRefresh:   clusterRefresh,
		VMs:              vms,
		VMDetail:         vmDetail,
		VMCloudInit:      vmCloudInit,
		VMCreate:         vmCreate,
		Tasks:            tasks,
		Auth:             authHandler,
		WebBuildDir:      webDir,
		Log:              logger,
		SnapshotHandlers: []*httpapi.VMSnapshots{snapshots},
		VMConsole:        vmConsole,
		AdminCatalog:     adminCatalog,
		AdminPolicy:      adminPolicy,
		AdminPools:       adminPools,
		AdminOps:         adminOps,
		AdminClusters:    adminClusters,
	}), nil
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
