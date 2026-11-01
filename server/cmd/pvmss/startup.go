//nolint:wsl_v5 // composition root keeps dependency wiring readable
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/docs/seed"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/telemetry"
	"pvmss/server/internal/vm"
	"time"
)

// loadConfig reads and validates environment configuration, then builds the
// structured logger. Returns a fallback stderr logger on config failure so the
// caller can log the error before exiting.
func loadConfig(stderr *slog.Logger) (config.Configuration, *slog.Logger, *slog.LevelVar, io.Closer, error) {
	cfg, err := config.Load()
	if err != nil {
		stderr.Error("failed to load configuration", "component", "main", "error", err)
		return config.Configuration{}, nil, nil, nil, err
	}

	logger, logLevel, logCloser, err := config.NewLogger(cfg)
	if err != nil {
		stderr.Error("failed to create logger", "component", "main", "error", err)
		return config.Configuration{}, nil, nil, nil, err
	}

	// Outermost wrapper: records logged with a span in their context gain
	// traceId/spanId. It sits above the level gate and redaction handlers.
	logger = slog.New(telemetry.LogHandler(logger.Handler()))
	slog.SetDefault(logger)
	logger.Debug("configuration loaded", "component", "main", "host", cfg.Host, "port", cfg.Port, "dbPath", cfg.DBPath)

	for _, key := range cfg.DeprecatedSSHEnv {
		logger.Warn("deprecated environment variable ignored", "component", "main", "env", key, "hint", "PVMSS no longer publishes cloud-init files over SSH: paste them on the nodes from Admin > Cloud-init (docs/cloud-init.md)")
	}
	return cfg, logger, logLevel, logCloser, nil
}

// openStore opens the SQLite database, runs migrations, and seeds built-in
// documentation pages (idempotent: only inserts missing rows).
func openStore(cfg config.Configuration, logger *slog.Logger) (*store.Store, error) {
	st, err := store.Open(cfg)
	if err != nil && cfg.DBResetOnIncompatible && errors.Is(err, store.ErrIncompatibleSchema) {
		st, err = reopenAfterQuarantine(cfg, logger, err)
	}
	if err != nil {
		logger.Error("failed to open database", "component", "main", "error", err)
		return nil, err
	}
	st.SetLogger(logger)
	logger.Debug("database opened", "component", "main", "migrationsDefined", len(store.Migrations))

	if err := seed.SeedDocumentationPages(context.Background(), st); err != nil {
		logger.Error("failed to seed documentation pages", "component", "main", "error", err)
		return nil, err
	}

	return st, nil
}

// reopenAfterQuarantine moves the incompatible database aside and opens a fresh
// one. The old file is kept, so nothing is ever lost to this recovery path.
func reopenAfterQuarantine(cfg config.Configuration, logger *slog.Logger, cause error) (*store.Store, error) {
	moved, err := store.QuarantineDatabase(cfg.DBPath, time.Now())
	if err != nil {
		return nil, errors.Join(cause, err)
	}
	logger.Warn("incompatible database moved aside, starting fresh", "component", "main", "path", cfg.DBPath, "movedTo", moved, "error", cause)

	return store.Open(cfg)
}

// initCluster builds the cluster registry from configured rows and resolves the
// default cluster client. The registry owns per-cluster Client instances; the
// default client is the one most handlers operate on.
//
// Display-name discovery is deferred to a background goroutine (see run) so a
// down cluster cannot block startup.
func initCluster(cfg config.Configuration, st *store.Store, logger *slog.Logger) (*cluster.Registry, cluster.Client, error) {
	rows, err := st.ListClusters(context.Background())
	if err != nil {
		logger.Error("failed to list configured clusters", "component", "main", "error", err)
		return nil, nil, err
	}
	clusterRegistry, err := cluster.NewRegistryWithLogger(cfg.ClusterSource, rows, logger)
	if err != nil {
		logger.Error("failed to create cluster registry", "component", "main", "error", err)
		return nil, nil, err
	}
	names := clusterRegistry.List()
	if len(names) == 0 {
		// Zero clusters is the first-run state when PVMSS_CLUSTER_SOURCE=proxmox
		// carries no PROXMOX_* credentials: the server boots in setup mode and
		// the wizard registers the first cluster over the API.
		logger.Info("no clusters configured, starting in setup mode", "component", "main")
		return clusterRegistry, nil, nil
	}
	clusterClient, err := clusterRegistry.Client(names[0])
	if err != nil {
		logger.Error("primary cluster is unavailable", "component", "main", "cluster", names[0], "error", err)
		return nil, nil, err
	}
	logger.Debug("cluster registry initialized", "component", "cluster", "source", cfg.ClusterSource, "clusters", clusterRegistry.List())
	return clusterRegistry, clusterClient, nil
}

// displayNameDiscoveryTimeout caps how long a single cluster's DisplayName
// call may take during background discovery. A dead cluster's GET is retried
// 3× with a 20s client timeout; without this cap one unreachable cluster
// would block discovery (and previously, startup) for up to 60s. 5s is
// enough for a healthy cluster's /cluster/status and bounds the dead case.
const displayNameDiscoveryTimeout = 5 * time.Second

// discoverClusterDisplayNames populates the display_name column for Proxmox
// clusters that don't already have one by calling Client.DisplayName() (the
// real Proxmox cluster name from /cluster/status). Fake clusters are skipped:
// their DisplayName() implementation just returns the internal logical name
// ("default", "secondary"), which is the opposite of a human-readable label -
// the fake seed already sets meaningful display names.
//
// Runs in a background goroutine launched after the HTTP server starts
// listening, so a down cluster cannot delay boot. Each cluster's call is
// bounded by displayNameDiscoveryTimeout.
func discoverClusterDisplayNames(ctx context.Context, registry *cluster.Registry, st *store.Store, logger *slog.Logger) {
	logger.DebugContext(ctx, "display name discovery started", "component", "cluster")
	defer logger.DebugContext(ctx, "display name discovery finished", "component", "cluster")

	rows, err := st.ListClusters(ctx)
	if err != nil {
		logger.WarnContext(ctx, "display name discovery: list clusters failed", "component", "cluster", "error", err)
		return
	}
	for _, row := range rows {
		if row.DisplayName != "" {
			continue
		}
		client, err := registry.Client(row.Name)
		if err != nil {
			logger.WarnContext(ctx, "display name discovery skipped: cluster not in registry", "component", "cluster", "cluster", row.Name, "error", err)
			continue
		}
		if _, ok := client.(cluster.Fake); ok {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, displayNameDiscoveryTimeout)
		displayName, err := client.DisplayName(callCtx)
		cancel()
		if err != nil {
			logger.WarnContext(ctx, "cluster display name discovery failed", "component", "cluster", "cluster", row.Name, "error", err)
			continue
		}
		if displayName == "" {
			continue
		}
		if err := st.SetClusterDisplayName(ctx, row.Name, displayName); err != nil {
			logger.WarnContext(ctx, "cluster display name persist failed", "component", "cluster", "cluster", row.Name, "error", err)
		}
	}
}

// initInventory builds the inventory registry and resolves the first
// cluster's projection, worker, and refresher. Each active cluster gets an
// independent projection and refresh worker.
func initInventory(cfg config.Configuration, clusterRegistry *cluster.Registry, logger *slog.Logger) (*inventory.Registry, *inventory.Projection, vm.IndexRefresher, *inventory.Refresher, error) {
	inventoryRegistry := inventory.NewRegistry(
		clusterRegistry,
		cfg.InventoryRefreshInterval,
		logger,
		inventory.WithRefreshTimeout(cfg.InventoryRefreshTimeout),
	)
	inventoryRegistry.SetManualRefreshMinInterval(cfg.InventoryManualRefreshMinInterval)

	names := clusterRegistry.List()
	if len(names) == 0 {
		return inventoryRegistry, nil, nil, nil, nil
	}
	defaultProjection, defaultWorker, defaultRefresher, err := resolvePrimaryInventory(inventoryRegistry, names[0])
	if err != nil {
		logger.Error("primary inventory is unavailable", "component", "main", "cluster", names[0], "error", err)
		return nil, nil, nil, nil, err
	}

	return inventoryRegistry, defaultProjection, defaultWorker, defaultRefresher, nil
}

// resolvePrimaryInventory resolves the projection, per-call worker, and
// refresher of the registry's primary cluster. Shared by startup and the
// setup-mode router swap, which performs the same resolution lazily once the
// wizard has registered the first cluster.
func resolvePrimaryInventory(inventoryRegistry *inventory.Registry, primary string) (*inventory.Projection, vm.IndexRefresher, *inventory.Refresher, error) {
	defaultProjection, err := inventoryRegistry.Projection(primary)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := inventoryRegistry.Worker(primary); err != nil {
		return nil, nil, nil, err
	}
	// The worker is rebuilt on every admin cluster edit; resolve it per call.
	defaultWorker := registryClusterRefresh{registry: inventoryRegistry, name: primary}
	defaultRefresher, err := inventoryRegistry.Refresher(primary)
	if err != nil {
		return nil, nil, nil, err
	}
	return defaultProjection, defaultWorker, defaultRefresher, nil
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
