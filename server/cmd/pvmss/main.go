// Command pvmss is the next-gen PVMSS server entry point: it wires the
// config, store, cluster client, inventory projection and HTTP API together
// and serves the SPA + REST endpoints on a single port.
//
//nolint:wsl_v5 // composition root keeps dependency wiring readable
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/config"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/telemetry"
	"pvmss/server/internal/vm"
	"runtime/debug"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
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
	// request. The write timeout is the outer ceiling for any handler,
	// including cluster calls.
	// 2. Handler-level: every handler that calls the cluster client passes
	// r.Context(), so cluster operations inherit the server's write
	// deadline and are cancelled when it fires.
	// 3. Worker-level: inventory refreshCycle wraps its cluster call in
	// context.WithTimeout(ctx, cfg.InventoryRefreshTimeout) so a hung
	// upstream cannot hold the singleflight lock indefinitely.
	//
	// Authenticated writes (VM actions, snapshots, bulk, manual refresh) can
	// outlast writeTimeout (lock retry 30s, refresh timeout 15s): the router's
	// withLongWriteDeadline lifts it per request for them.

	// appVersion is the version string surfaced in the dashboard, the admin
	// app info page, and the public /api/v1/public/version endpoint.
	// It is a compile-time literal; no runtime discovery is performed.
	appVersion = "0.4.0"

	// errNoClustersConfigured is the error message shared by initCluster and
	// initInventory when the registry has no clusters. Extracted as a
	// constant to satisfy go:S1192 (the literal was duplicated 4×).
	errNoClustersConfigured = "no clusters configured"
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

	cfg, logger, logLevel, logCloser, err := loadConfig(stderr)
	if err != nil {
		return 1
	}
	defer func() { _ = logCloser.Close() }()

	bi, _ := debug.ReadBuildInfo()

	tel, err := telemetry.Setup(context.Background(), telemetry.Config{
		Version: appVersion, Commit: vcsSetting(bi, "vcs.revision"), Metrics: cfg.MetricsPort != 0,
	}, os.Getenv)
	if err != nil {
		logger.Warn("telemetry disabled: setup failed", "component", "main", "error", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tel.Shutdown(ctx); err != nil {
			logger.Warn("telemetry flush failed", "component", "main", "error", err)
		}
	}()

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
	logger.Debug("web build directory resolved", "component", "main", "webDir", webDir)

	clusterRegistry, clusterClient, err := initCluster(cfg, st, logger)
	if err != nil {
		return 1
	}

	inventoryRegistry, defaultProjection, defaultWorker, defaultRefresher, err := initInventory(cfg, clusterRegistry, logger)
	if err != nil {
		return 1
	}

	logBanner(logger, cfg, clusterRegistry.List(), bi, telemetry.EndpointHost(os.Getenv))
	logTLSAdvice(logger, cfg)

	if _, err := telemetry.RegisterInventoryGauges(otel.Meter("pvmss"), inventoryStats(inventoryRegistry)); err != nil {
		logger.Warn("inventory gauges not registered", "component", "main", "error", err)
	}

	stopMetrics, err := startMetricsServer(cfg, tel.MetricsHandler, logger)
	if err != nil {
		logger.Error("metrics server failed to start", "component", "main", "error", err)
		return 1
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = stopMetrics(ctx)
	}()

	// Start every worker before the HTTP server accepts traffic so the
	// projections are populated before the first request can arrive.
	inventoryCtx, cancelInventory := context.WithCancel(context.Background())

	defer cancelInventory()

	inventoryRegistry.Start(inventoryCtx)

	// The one policy service shared by the router and the background
	// sweeps: projections are resolved per cluster through the inventory
	// registry, so quota and name checks never read another cluster's VMs.
	policyService := policy.New(st, nil)
	policyService.SetClusterProvider(clusterRegistry)
	policyService.SetProjectionSource(inventoryRegistry)
	policyService.SetQuotaAuditor(st)

	// Daily audit prune tick: deletes audit_log rows older than
	// the configured retention. Runs in its own goroutine so it does not
	// couple to the inventory worker's refresh cycle. A prune runs once at
	// startup, then every 24h.
	go runAuditPrune(inventoryCtx, st, logger)

	// Daily lease sweep: stops and deletes VMs whose lease expired. Runs once
	// at startup, then every 24h, like the audit prune.
	go runLeaseSweep(inventoryCtx, st, clusterRegistry, inventoryRegistry, logger)

	// Scheduled snapshots: a sweep every snapshotSweepInterval takes the due
	// automatic snapshots and prunes them to each VM's retention.
	go runSnapshotSweep(inventoryCtx, vm.SweepDependencies{
		Store:   st,
		Indexes: inventoryRegistry,
		Clients: clusterRegistry,
		Policy:  policyService,
		Log:     logger,
		Now:     time.Now,
	})

	sessions, err := auth.NewSessionManager(st, cfg.SessionSecret, cfg.CookieSecure)
	if err != nil {
		logger.Error("failed to create session manager", "component", "main", "error", err)
		return 1
	}

	router, err := buildRouter(routerDeps{cfg: cfg, clusterRegistry: clusterRegistry, inventoryRegistry: inventoryRegistry, clusterClient: clusterClient, projection: defaultProjection, refresher: defaultRefresher, worker: defaultWorker, sessions: sessions, st: st, policy: policyService, webDir: webDir, logger: logger, logLevel: logLevel})
	if err != nil {
		logger.Error("failed to build router", "component", "main", "error", err)
		return 1
	}

	// Discover cluster display names in the background so a down cluster
	// cannot block startup. DisplayName calls a GET (retried 3× with a 20s
	// client timeout) - a dead cluster would hang the server for up to 60s
	// per cluster before it ever started listening. The 5s per-cluster cap
	// bounds the worst case; a successful discovery updates the row and the
	// sidebar picks it up on the next /me or page load.
	go discoverClusterDisplayNames(context.Background(), clusterRegistry, st, logger)

	return serve(router, cfg, logger)
}

// serve starts the HTTP server and blocks until it stops via error or signal.
// On SIGINT/SIGTERM it gives the server 5 s to drain in-flight requests.
func serve(router http.Handler, cfg config.Configuration, logger *slog.Logger) int {
	srv := &http.Server{
		Addr:              listenAddr(cfg),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// Failed handshakes (missing client cert, ...) go to slog, not stderr.
		ErrorLog: slog.NewLogLogger(logger.With("component", "http").Handler(), slog.LevelWarn),
	}

	tlsCfg, err := cfg.ServerTLSConfig()
	if err != nil {
		logger.Error("server tls config failed", "component", "main", "error", err)
		return 1
	}

	srv.TLSConfig = tlsCfg

	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", srv.Addr)
	if err != nil {
		logger.Error("server listen failed", "component", "main", "addr", srv.Addr, "error", err)
		return 1
	}

	logger.Info("server ready", "component", "main", "addr", srv.Addr)

	errCh := make(chan error, 1)
	go func() {
		if tlsCfg != nil {
			errCh <- srv.ServeTLS(ln, "", "")
			return
		}
		errCh <- srv.Serve(ln)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var drainStart time.Time

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "component", "main", "error", err)
			return 1
		}
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "component", "main", "signal", sig.String())

		drainStart = time.Now()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "component", "main", "error", err)
			return 1
		}
	}

	logger.Info("server stopped", "component", "main", "durationMs", time.Since(drainStart).Milliseconds())

	return 0
}
