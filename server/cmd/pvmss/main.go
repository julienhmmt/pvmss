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

	// Daily audit prune tick (issue #02): deletes audit_log rows older than
	// the configured retention. Runs in its own goroutine so it does not
	// couple to the inventory worker's refresh cycle. A prune runs once at
	// startup, then every 24h.
	go runAuditPrune(inventoryCtx, st, logger)

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
//
// It also discovers each cluster's display name (the real Proxmox cluster name
// from /cluster/status, or the fake's logical name) and persists it when the
// row does not already have one — so the sidebar shows a meaningful name from
// first boot instead of the internal "default" until an admin tests the
// cluster. Discovery is best-effort: a failed DisplayName call logs a warning
// and leaves the row untouched, never blocking startup.
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
	discoverClusterDisplayNames(context.Background(), clusterRegistry, rows, st, logger)
	logger.Info("cluster registry initialized", "component", "cluster", "source", cfg.ClusterSource, "clusters", clusterRegistry.List())
	return clusterRegistry, clusterClient, nil
}

// discoverClusterDisplayNames populates the display_name column for Proxmox
// clusters that don't already have one by calling Client.DisplayName() (the
// real Proxmox cluster name from /cluster/status). Fake clusters are skipped:
// their DisplayName() implementation just returns the internal logical name
// ("default", "secondary"), which is the opposite of a human-readable label —
// the fake seed already sets meaningful display names.
func discoverClusterDisplayNames(ctx context.Context, registry *cluster.Registry, rows []store.ClusterRow, st *store.Store, logger *slog.Logger) {
	for _, row := range rows {
		if row.DisplayName != "" {
			continue
		}
		client, err := registry.Client(row.Name)
		if err != nil {
			logger.Warn("display name discovery skipped: cluster not in registry", "component", "cluster", "cluster", row.Name, "error", err)
			continue
		}
		if _, ok := client.(cluster.Fake); ok {
			continue
		}
		displayName, err := client.DisplayName(ctx)
		if err != nil {
			logger.Warn("cluster display name discovery failed", "component", "cluster", "cluster", row.Name, "error", err)
			continue
		}
		if displayName == "" {
			continue
		}
		if err := st.SetClusterDisplayName(ctx, row.Name, displayName); err != nil {
			logger.Warn("cluster display name persist failed", "component", "cluster", "cluster", row.Name, "error", err)
		}
	}
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

	clients, err := resolveClusterClientInterfaces(clusterClient)
	if err != nil {
		return nil, err
	}

	consoleTickets := vm.NewConsoleTicketStore()

	// Every *WithRegistry constructor below resolves its cluster.Client and
	// inventory.Index per request from the request's own :cluster value via
	// clusterRegistry/inventoryRegistry, instead of the single clusterClient
	// resolved above — a request scoped to a non-default cluster must never
	// be served from another cluster's client (cross-tenant data leak when
	// node names or vmids collide between clusters).
	vmDetail := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Writer: clients.writer, Clients: clusterRegistry, Store: st, Refresher: worker, Log: logger}, policyService)
	vmBulk := httpapi.NewVMBulkWithRegistry(httpapi.VMBulkRegistryDeps{Registry: inventoryRegistry, Projection: projection, Auth: authHandler, Writer: clients.writer, Store: st, Refresher: refresher, Log: logger, Clients: clusterRegistry})
	vmCloudInit := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Reader: clients.cloudInitReader, Writer: clients.writer, Clients: clusterRegistry, Store: st, Refresher: worker, Log: logger}, policyService)
	vmCreate := httpapi.NewVMCreateWithRegistry(
		authHandler,
		st,
		clusterRegistry,
		clients.creator,
		clients.writer,
		logger,
		policyService,
	)
	vmCreate.SetTrustedProxyHops(cfg.TrustedProxyHops)
	tasks := httpapi.NewTasksWithRegistry(authHandler, clusterRegistry, clients.creator, worker, logger)
	snapshots := httpapi.NewVMSnapshotsWithRegistry(httpapi.VMSnapshotsRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Reader: clients.snapshotReader, Writer: clients.snapshotWriter, Clients: clusterRegistry, Store: st, Log: logger, Services: []*policy.Policy{policyService}})
	vmConsole := httpapi.NewVMConsoleWithRegistry(httpapi.VMConsoleRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Relay: clients.consoleRelay, Clients: clusterRegistry, Tickets: consoleTickets, Store: st, Log: logger})
	vmSerialConsole := httpapi.NewVMSerialConsoleWithRegistry(httpapi.VMSerialConsoleRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Relay: clients.serialRelay, Clients: clusterRegistry, Tickets: consoleTickets, Store: st, Log: logger})
	vmMetrics := httpapi.NewVMMetricsWithRegistry(inventoryRegistry, projection, authHandler, clients.metricsReader, clients.metricsCurrentReader, clusterRegistry, logger)
	adminCatalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, clusterRegistry, projection, logger)
	adminCatalog.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminPolicy := httpapi.NewAdminPolicyWithRegistry(authHandler, policyService, clusterRegistry, logger)
	adminPolicy.SetStore(st)
	adminPolicy.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminPools := httpapi.NewAdminPoolsWithRegistry(httpapi.AdminPoolsRegistryDeps{Auth: authHandler, Clients: clusterRegistry, Source: inventoryRegistry, Projection: projection, Writer: clients.writer, Audit: st, Refresher: worker, Store: st, Log: logger})
	adminPools.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminOps := httpapi.NewAdminOps(authHandler, st, clusterClient, projection, appVersion, logger)
	adminOps.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminClusters := httpapi.NewAdminClusters(authHandler, st, clusterRegistry, inventoryRegistry, logger)
	adminClusters.SetTrustedProxyHops(cfg.TrustedProxyHops)
	docsHandler := httpapi.NewDocsAPIHandler(authHandler, st, logger)
	adminDocs := httpapi.NewAdminDocs(authHandler, st, docsHandler, logger)

	authHandler.SetTrustedProxyHops(cfg.TrustedProxyHops)
	vm.SetResolveAuditor(st)
	policy.SetQuotaAuditor(st)

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
		Store:            st,
		SnapshotHandlers: []*httpapi.VMSnapshots{snapshots},
		VMConsole:        vmConsole,
		VMSerialConsole:  vmSerialConsole,
		VMMetrics:        vmMetrics,
		AdminCatalog:     adminCatalog,
		AdminPolicy:      adminPolicy,
		AdminPools:       adminPools,
		AdminOps:         adminOps,
		AdminClusters:    adminClusters,
		Docs:             docsHandler,
		AdminDocs:        adminDocs,
		TrustedProxyHops: cfg.TrustedProxyHops,
	}), nil
}

// clusterClientInterfaces bundles the cluster.Client capability interfaces
// resolved once at router build time.
type clusterClientInterfaces struct {
	writer               cluster.Writer
	creator              cluster.Creator
	cloudInitReader      cluster.CloudInitReader
	snapshotReader       cluster.SnapshotReader
	snapshotWriter       cluster.SnapshotWriter
	consoleRelay         cluster.ConsoleRelay
	serialRelay          cluster.TerminalRelay
	metricsReader        cluster.MetricsHistoryReader
	metricsCurrentReader cluster.MetricsCurrentReader
}

// resolveClusterClientInterfaces asserts that the cluster client implements
// every capability interface the router needs. Both cluster.Client
// implementations (Fake, Proxmox) satisfy all of them — reads and writes are
// separated by interface (constitution IV), not by implementation.
func resolveClusterClientInterfaces(clusterClient cluster.Client) (clusterClientInterfaces, error) {
	var c clusterClientInterfaces
	var ok bool

	if c.writer, ok = clusterClient.(cluster.Writer); !ok {
		return c, errors.New("cluster client does not implement Writer")
	}
	if c.creator, ok = clusterClient.(cluster.Creator); !ok {
		return c, errors.New("cluster client does not implement Creator")
	}
	if c.cloudInitReader, ok = clusterClient.(cluster.CloudInitReader); !ok {
		return c, errors.New("cluster client does not implement CloudInitReader")
	}
	if c.snapshotReader, ok = clusterClient.(cluster.SnapshotReader); !ok {
		return c, errors.New("cluster client does not implement SnapshotReader")
	}
	if c.snapshotWriter, ok = clusterClient.(cluster.SnapshotWriter); !ok {
		return c, errors.New("cluster client does not implement SnapshotWriter")
	}
	if c.consoleRelay, ok = clusterClient.(cluster.ConsoleRelay); !ok {
		return c, errors.New("cluster client does not implement ConsoleRelay")
	}
	if c.serialRelay, ok = clusterClient.(cluster.TerminalRelay); !ok {
		return c, errors.New("cluster client does not implement TerminalRelay")
	}
	if c.metricsReader, ok = clusterClient.(cluster.MetricsHistoryReader); !ok {
		return c, errors.New("cluster client does not implement MetricsHistoryReader")
	}
	if c.metricsCurrentReader, ok = clusterClient.(cluster.MetricsCurrentReader); !ok {
		return c, errors.New("cluster client does not implement MetricsCurrentReader")
	}

	return c, nil
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

// auditPruneInterval is how often the daily prune tick fires.
const auditPruneInterval = 24 * time.Hour

// runAuditPrune deletes audit_log rows older than the configured retention,
// once at startup then every auditPruneInterval. It logs the deleted count at
// info level so retention activity is visible in aggregated logs. A prune
// failure is logged but never stops the tick — the next tick retries.
func runAuditPrune(ctx context.Context, st *store.Store, log *slog.Logger) {
	prune := func() {
		cfg, err := st.GetAuditConfig(ctx)
		if err != nil {
			log.Error("audit prune: get config failed", "component", "audit", "error", err)
			return
		}

		n, err := st.PruneAuditLog(ctx, cfg.RetentionDays)
		if err != nil {
			log.Error("audit prune failed", "component", "audit", "error", err)
			return
		}

		if n > 0 {
			log.Info("audit prune completed", "component", "audit", "deleted", n, "retentionDays", cfg.RetentionDays)
		}
	}

	prune()

	ticker := time.NewTicker(auditPruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}
