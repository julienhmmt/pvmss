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
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/docs/seed"
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

	// Timeout strategy (defence in depth):
	//
	// 1. Server-level (here): readHeader, read, write, idle bound every
	//    request. The write timeout is the outer ceiling for any handler,
	//    including cluster calls.
	// 2. Handler-level: every handler that calls the cluster client passes
	//    r.Context(), so cluster operations inherit the server's write
	//    deadline and are cancelled when it fires.
	// 3. Worker-level: inventory refreshCycle wraps its cluster call in
	//    context.WithTimeout(ctx, cfg.InventoryRefreshTimeout) so a hung
	//    upstream cannot hold the singleflight lock indefinitely.
	//
	// Note: InventoryRefreshTimeout (default 15s) can exceed writeTimeout
	// (10s). With the fake cluster this is harmless (calls are instant).
	// When a real Proxmox client is implemented, either raise writeTimeout
	// to >= InventoryRefreshTimeout or make the manual-refresh handler
	// asynchronous (202 Accepted + background refresh).

	// appVersion is the version string surfaced in the dashboard, the admin
	// app info page, and the public /api/v1/public/version endpoint (T14).
	// It is a compile-time literal; no runtime discovery is performed.
	appVersion = "0.4.0-dev"
)

func main() {
	os.Exit(run())
}

// run is the composition root: it wires every runtime dependency in startup
// order and drives the server lifecycle. Each phase is extracted into a named
// function so the wiring sequence stays readable; defers for closeable
// resources remain here so they always fire on any return path.
func run() int {
	stderr := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg, logger, logCloser, err := loadConfig(stderr)
	if err != nil {
		return 1
	}
	defer func() { _ = logCloser.Close() }()

	st, err := openStore(cfg, logger)
	if err != nil {
		return 1
	}
	defer func() { _ = st.Close() }()

	webDir, err := resolveWebBuildDir(cfg.WebDir)
	if err != nil {
		logger.Error("web build directory not found", "component", "main", "error", err)
		return 1
	}
	logger.Info("web build directory resolved", "component", "main", "webDir", webDir)

	clusterRegistry, clusterClient, err := initCluster(cfg, st, logger)
	if err != nil {
		return 1
	}

	inventoryRegistry, defaultProjection, defaultWorker, defaultRefresher, err := initInventory(cfg, clusterRegistry, logger)
	if err != nil {
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

	router, err := buildRouter(routerDeps{cfg: cfg, clusterRegistry: clusterRegistry, inventoryRegistry: inventoryRegistry, clusterClient: clusterClient, projection: defaultProjection, refresher: defaultRefresher, worker: defaultWorker, sessions: sessions, st: st, webDir: webDir, logger: logger})
	if err != nil {
		logger.Error("failed to build router", "component", "main", "error", err)
		return 1
	}

	return serve(router, cfg, logger)
}

// loadConfig reads and validates environment configuration, then builds the
// structured logger. Returns a fallback stderr logger on config failure so the
// caller can log the error before exiting.
func loadConfig(stderr *slog.Logger) (config.Configuration, *slog.Logger, io.Closer, error) {
	cfg, err := config.Load()
	if err != nil {
		stderr.Error("failed to load configuration", "component", "main", "error", err)
		return config.Configuration{}, nil, nil, err
	}

	logger, logCloser, err := config.NewLogger(cfg)
	if err != nil {
		stderr.Error("failed to create logger", "component", "main", "error", err)
		return config.Configuration{}, nil, nil, err
	}

	logger.Info("configuration loaded", "component", "main", "host", cfg.Host, "port", cfg.Port, "dbPath", cfg.DBPath)
	return cfg, logger, logCloser, nil
}

// openStore opens the SQLite database, runs migrations, and seeds built-in
// documentation pages (issue #53 — idempotent: only inserts missing rows).
func openStore(cfg config.Configuration, logger *slog.Logger) (*store.Store, error) {
	st, err := store.Open(cfg)
	if err != nil {
		logger.Error("failed to open database", "component", "main", "error", err)
		return nil, err
	}
	logger.Info("database opened", "component", "main", "migrationsDefined", len(store.Migrations))

	if err := seed.SeedDocumentationPages(context.Background(), st); err != nil {
		logger.Error("failed to seed documentation pages", "component", "main", "error", err)
		return nil, err
	}

	return st, nil
}

// initCluster builds the cluster registry from configured rows and resolves the
// default cluster client. The registry owns per-cluster Client instances; the
// default client is the one most handlers operate on.
func initCluster(cfg config.Configuration, st *store.Store, logger *slog.Logger) (*cluster.Registry, cluster.Client, error) {
	rows, err := st.ListClusters(context.Background())
	if err != nil {
		logger.Error("failed to list configured clusters", "component", "main", "error", err)
		return nil, nil, err
	}
	clusterRegistry, err := cluster.NewRegistry(cfg.ClusterSource, rows)
	if err != nil {
		logger.Error("failed to create cluster registry", "component", "main", "error", err)
		return nil, nil, err
	}
	clusterClient, err := clusterRegistry.Client("default")
	if err != nil {
		logger.Error("default cluster is unavailable", "component", "main", "error", err)
		return nil, nil, err
	}
	logger.Info("cluster registry initialized", "component", "cluster", "source", cfg.ClusterSource, "clusters", clusterRegistry.List())
	return clusterRegistry, clusterClient, nil
}

// initInventory builds the inventory registry and resolves the default
// cluster's projection, worker, and refresher. Each active cluster gets an
// independent projection and refresh worker (FR-002, AC03 §2.4).
func initInventory(cfg config.Configuration, clusterRegistry *cluster.Registry, logger *slog.Logger) (*inventory.Registry, *inventory.Projection, *inventory.Worker, *inventory.Refresher, error) {
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
		return nil, nil, nil, nil, err
	}
	defaultWorker, err := inventoryRegistry.Worker("default")
	if err != nil {
		logger.Error("default inventory worker is unavailable", "component", "main", "error", err)
		return nil, nil, nil, nil, err
	}
	defaultRefresher, err := inventoryRegistry.Refresher("default")
	if err != nil {
		logger.Error("default inventory refresher is unavailable", "component", "main", "error", err)
		return nil, nil, nil, nil, err
	}

	return inventoryRegistry, defaultProjection, defaultWorker, defaultRefresher, nil
}

// serve starts the HTTP server and blocks until it stops via error or signal.
// On SIGINT/SIGTERM it gives the server 5 s to drain in-flight requests.
func serve(router http.Handler, cfg config.Configuration, logger *slog.Logger) int {
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

// routerDeps groups the shared dependencies for building the HTTP router.
// It collapses the eleven positional parameters buildRouter used to take
// (SonarQube go:S107).
type routerDeps struct {
	cfg               config.Configuration
	clusterRegistry   *cluster.Registry
	inventoryRegistry *inventory.Registry
	clusterClient     cluster.Client
	projection        *inventory.Projection
	refresher         *inventory.Refresher
	worker            *inventory.Worker
	sessions          *auth.SessionManager
	st                *store.Store
	webDir            string
	logger            *slog.Logger
}

// buildRouter wires all HTTP handlers into the final router. It performs the
// cluster.Writer/Creator type assertions (both Fake and Proxmox satisfy them)
// and constructs the handler graph from the shared dependencies.
func buildRouter(deps routerDeps) (http.Handler, error) {
	cfg := deps.cfg
	clusterRegistry := deps.clusterRegistry
	inventoryRegistry := deps.inventoryRegistry
	clusterClient := deps.clusterClient
	projection := deps.projection
	refresher := deps.refresher
	worker := deps.worker
	sessions := deps.sessions
	st := deps.st
	webDir := deps.webDir
	logger := deps.logger
	policyService := policy.New(st, projection, clusterClient)
	health := httpapi.NewHealth(st, logger, inventoryFreshness{registry: inventoryRegistry, demoMode: cfg.ClusterSource == "fake"}, 2*cfg.InventoryRefreshInterval)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(refresher, logger)
	authHandler := httpapi.NewAuthWithRegistry(clusterRegistry, st, sessions, cfg.AdminPasswordHash, auth.NewTokenService(st), logger)
	vms := httpapi.NewVMsWithRegistry(inventoryRegistry, authHandler, cfg.MaxListPageSize, 0, logger, st, policyService)

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

	metricsReader, ok := clusterClient.(cluster.MetricsHistoryReader)
	if !ok {
		return nil, errors.New("cluster client does not implement MetricsHistoryReader")
	}

	vmDetail := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Writer: writer, Store: st, Refresher: worker, Log: logger}, policyService)
	vmBulk := httpapi.NewVMBulkWithRegistry(inventoryRegistry, projection, authHandler, writer, st, refresher, logger)
	vmCloudInit := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{Projection: projection, Auth: authHandler, Reader: cloudInitReader, Writer: writer, Store: st, Refresher: worker, Log: logger}, policyService)
	vmCreate := httpapi.NewVMCreateWithRegistry(
		authHandler,
		st,
		clusterRegistry,
		creator,
		writer,
		logger,
		policyService,
	)
	tasks := httpapi.NewTasks(authHandler, creator, worker, logger)
	snapshots := httpapi.NewVMSnapshots(projection, authHandler, snapshotReader, snapshotWriter, st, logger, policyService)
	vmConsole := httpapi.NewVMConsole(projection, authHandler, consoleRelay, consoleTickets, st, logger)
	vmMetrics := httpapi.NewVMMetrics(projection, authHandler, metricsReader, logger)
	adminCatalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, clusterRegistry, projection, logger)
	adminPolicy := httpapi.NewAdminPolicyWithRegistry(authHandler, policyService, clusterRegistry, logger)
	adminPools := httpapi.NewAdminPools(authHandler, clusterClient, projection, writer, st, worker, st, logger)
	adminOps := httpapi.NewAdminOps(authHandler, st, clusterClient, projection, appVersion, logger)
	adminClusters := httpapi.NewAdminClusters(authHandler, st, clusterRegistry, inventoryRegistry, logger)
	docsHandler := httpapi.NewDocsAPIHandler(authHandler, st, logger)
	adminDocs := httpapi.NewAdminDocs(authHandler, st, docsHandler, logger)

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health:           health,
		ClusterNodes:     clusterNodes,
		ClusterRefresh:   clusterRefresh,
		VMs:              vms,
		VMDetail:         vmDetail,
		VMBulk:           vmBulk,
		VMCloudInit:      vmCloudInit,
		VMCreate:         vmCreate,
		Tasks:            tasks,
		Auth:             authHandler,
		WebBuildDir:      webDir,
		Log:              logger,
		SnapshotHandlers: []*httpapi.VMSnapshots{snapshots},
		VMConsole:        vmConsole,
		VMMetrics:        vmMetrics,
		AdminCatalog:     adminCatalog,
		AdminPolicy:      adminPolicy,
		AdminPools:       adminPools,
		AdminOps:         adminOps,
		AdminClusters:    adminClusters,
		Docs:             docsHandler,
		AdminDocs:        adminDocs,
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

// inventoryFreshness adapts inventory.Registry to httpapi.ClusterFreshnessChecker.
// It reads each cluster's Index.RefreshedAt (already maintained by the refresh
// goroutines) and the demoMode flag — zero cluster.Client calls from the health
// handler (constitution IV, FR-010).
type inventoryFreshness struct {
	registry *inventory.Registry
	demoMode bool
}

func (f inventoryFreshness) Clusters() []httpapi.ClusterFreshness {
	all := f.registry.All()
	result := make([]httpapi.ClusterFreshness, 0, len(all))
	for name, index := range all {
		refreshedAt := time.Time{}
		if index != nil {
			refreshedAt = index.RefreshedAt
		}
		result = append(result, httpapi.ClusterFreshness{Name: name, RefreshedAt: refreshedAt})
	}
	return result
}

func (f inventoryFreshness) DemoMode() bool {
	return f.demoMode
}
