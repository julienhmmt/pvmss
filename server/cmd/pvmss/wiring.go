package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"time"
)

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
	worker            vm.IndexRefresher
	sessions          *auth.SessionManager
	st                *store.Store
	policy            *policy.Policy
	webDir            string
	logger            *slog.Logger
	logLevel          *slog.LevelVar
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
	worker := deps.worker
	sessions := deps.sessions
	st := deps.st
	webDir := deps.webDir
	logger := deps.logger
	policyService := deps.policy
	freshness := inventoryFreshness{registry: inventoryRegistry, demoMode: cfg.ClusterSource == "fake"}
	health := httpapi.NewHealth(st, logger, freshness, 2*cfg.InventoryRefreshInterval)
	authHandler := httpapi.NewAuthWithRegistry(clusterRegistry, st, sessions, cfg.AdminPasswordHash, logger)
	authHandler.SetClusterFreshnessChecker(freshness, 2*cfg.InventoryRefreshInterval)
	health.SetAuthHandler(authHandler)
	clusterNodes := httpapi.NewClusterNodesWithRegistry(inventoryRegistry, authHandler, logger)
	clusterRefresh := httpapi.NewClusterRefreshWithRegistry(inventoryRegistry, clusterRegistry, authHandler, logger)
	vms := httpapi.NewVMsWithRegistry(inventoryRegistry, authHandler, cfg.MaxListPageSize, 0, logger, st, policyService)

	clients, err := resolveClusterClientInterfaces(clusterClient)
	if err != nil {
		return nil, err
	}

	consoleTickets := vm.NewConsoleTicketStore()

	// Every *WithRegistry constructor below resolves its cluster.Client and
	// inventory.Index per request from the request's own :cluster value via
	// clusterRegistry/inventoryRegistry, instead of the single clusterClient
	// resolved above - a request scoped to a non-default cluster must never
	// be served from another cluster's client (cross-tenant data leak when
	// node names or vmids collide between clusters).
	vmDetail := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Writer: clients.writer, Clients: clusterRegistry, Store: st, Refresher: worker, StatusReader: clients.statusReader, Creator: clients.creator, GuestNetReader: clients.guestNetReader, Log: logger}, policyService)
	vmBulk := httpapi.NewVMBulkWithRegistry(httpapi.VMBulkRegistryDeps{Registry: inventoryRegistry, Projection: projection, Auth: authHandler, Writer: clients.writer, Store: st, Refresher: worker, Log: logger, Clients: clusterRegistry})
	vmStatusBatch := httpapi.NewVMStatusBatch(httpapi.VMStatusBatchDeps{Source: inventoryRegistry, Auth: authHandler, StatusReader: clients.statusReader, Clients: clusterRegistry, Log: logger})
	vmCloudInit := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Reader: clients.cloudInitReader, Writer: clients.writer, StatusReader: clients.statusReader, Clients: clusterRegistry, Store: st, Refresher: worker, Log: logger}, policyService)
	vmCreate := httpapi.NewVMCreateWithRegistry(
		authHandler,
		st,
		clusterRegistry,
		clients.creator,
		logger,
		policyService,
	)
	vmCreate.SetTrustedProxyHops(cfg.TrustedProxyHops)
	vmCreate.SetInventoryRefreshers(httpapi.NewRegistryRefresherResolver(inventoryRegistry))
	vmCreate.SetInventorySource(inventoryRegistry)
	tasks := httpapi.NewTasksWithRegistry(authHandler, clusterRegistry, clients.creator, worker, httpapi.NewRegistryRefresherResolver(inventoryRegistry), logger)
	snapshots := httpapi.NewVMSnapshotsWithRegistry(httpapi.VMSnapshotsRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Reader: clients.snapshotReader, Writer: clients.snapshotWriter, Clients: clusterRegistry, Store: st, Log: logger, Policy: policyService})
	adminMigration := httpapi.NewAdminMigrationWithRegistry(httpapi.AdminMigrationRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Migrator: clients.migrator, Status: clients.statusReader, Clients: clusterRegistry, Store: st, Log: logger})
	vmConsole := httpapi.NewVMConsoleWithRegistry(httpapi.VMConsoleRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Relay: clients.consoleRelay, Clients: clusterRegistry, Tickets: consoleTickets, Store: st, Log: logger})
	vmSerialConsole := httpapi.NewVMSerialConsoleWithRegistry(httpapi.VMSerialConsoleRegistryDeps{Source: inventoryRegistry, Projection: projection, Auth: authHandler, Relay: clients.serialRelay, Clients: clusterRegistry, Tickets: consoleTickets, Store: st, Log: logger})
	vmMetrics := httpapi.NewVMMetricsWithRegistry(inventoryRegistry, projection, authHandler, clients.metricsReader, clients.metricsCurrentReader, clusterRegistry, logger)
	adminCatalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, clusterRegistry, projection, logger)
	adminCatalog.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminPolicy := httpapi.NewAdminPolicyWithRegistry(authHandler, policyService, clusterRegistry, logger)
	adminPolicy.SetStore(st)
	adminPolicy.SetTrustedProxyHops(cfg.TrustedProxyHops)
	quotaRequests := httpapi.NewQuotaRequests(authHandler, policyService, st, logger)
	quotaRequests.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminPools := httpapi.NewAdminPoolsWithRegistry(httpapi.AdminPoolsRegistryDeps{Auth: authHandler, Clients: clusterRegistry, Source: inventoryRegistry, Projection: projection, Writer: clients.writer, Audit: st, Refresher: worker, Store: st, Policy: policyService, Log: logger})
	adminPools.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminOps := httpapi.NewAdminOps(authHandler, st, projection, appVersion, logger)
	// The level at build time is the startup default (LOG_LEVEL); nothing has
	// changed it yet. The runtime endpoint can move it and reset back to it.
	adminOps.SetLogLevel(deps.logLevel, deps.logLevel.Level())
	adminOps.SetTrustedProxyHops(cfg.TrustedProxyHops)
	adminOps.SetInventorySource(inventoryRegistry, 2*cfg.InventoryRefreshInterval)
	adminClusters := httpapi.NewAdminClusters(authHandler, st, clusterRegistry, inventoryRegistry, logger)
	adminClusters.SetTrustedProxyHops(cfg.TrustedProxyHops)
	docsHandler := httpapi.NewDocsAPIHandler(authHandler, st, logger)
	adminDocs := httpapi.NewAdminDocs(authHandler, st, docsHandler, logger)
	adminDocs.SetTrustedProxyHops(cfg.TrustedProxyHops)
	profileSSHKeys := httpapi.NewProfileSSHKeys(authHandler, st, logger)
	adminAccounts := httpapi.NewAdminAccounts(authHandler, st, cfg.MaxLocalAdmins, logger)
	adminAccounts.SetTrustedProxyHops(cfg.TrustedProxyHops)

	authHandler.SetTrustedProxyHops(cfg.TrustedProxyHops)

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health:           health,
		ClusterNodes:     clusterNodes,
		ClusterRefresh:   clusterRefresh,
		VMs:              vms,
		VMDetail:         vmDetail,
		VMBulk:           vmBulk,
		VMStatusBatch:    vmStatusBatch,
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
		QuotaRequests:    quotaRequests,
		AdminPools:       adminPools,
		AdminOps:         adminOps,
		AdminClusters:    adminClusters,
		Docs:             docsHandler,
		AdminDocs:        adminDocs,
		ProfileSSHKeys:   profileSSHKeys,
		AdminMigration:   adminMigration,
		AdminAccounts:    adminAccounts,
		TrustedProxyHops: cfg.TrustedProxyHops,
		RateLimitMax:     cfg.RateLimitMax,
	}), nil
}

// registryClusterRefresh refreshes one cluster through the inventory
// registry, so it always drives the cluster's current worker - an admin edit
// replaces the worker and its client.
type registryClusterRefresh struct {
	registry *inventory.Registry
	name     string
}

func (r registryClusterRefresh) Refresh(ctx context.Context) (time.Time, error) {
	return r.registry.Refresh(ctx, r.name)
}

// clusterClientInterfaces bundles the cluster.Client capability interfaces
// resolved once at router build time.
type clusterClientInterfaces struct {
	writer               cluster.Writer
	creator              cluster.Creator
	cloudInitReader      cluster.CloudInitReader
	snapshotReader       cluster.SnapshotReader
	snapshotWriter       cluster.SnapshotWriter
	migrator             cluster.Migrator
	consoleRelay         cluster.ConsoleRelay
	serialRelay          cluster.TerminalRelay
	metricsReader        cluster.MetricsHistoryReader
	metricsCurrentReader cluster.MetricsCurrentReader
	statusReader         cluster.VMStatusReader
	guestNetReader       cluster.GuestNetworkReader
}

// must asserts the cluster client implements capability interface T. Every
// concrete client (Fake, Proxmox) satisfies all of them; a missing one is a
// wiring error, not a runtime condition.
func must[T any](client cluster.Client, name string) (T, error) {
	capability, ok := client.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("cluster client does not implement %s", name)
	}
	return capability, nil
}

// resolveClusterClientInterfaces asserts that the cluster client implements
// every capability interface the router needs. Both cluster.Client
// implementations (Fake, Proxmox) satisfy all of them - reads and writes are
// separated by interface, not by implementation.
func resolveClusterClientInterfaces(clusterClient cluster.Client) (clusterClientInterfaces, error) {
	var c clusterClientInterfaces
	var err error

	fields := []struct {
		name string
		set  func(v error) error
	}{}
	_ = fields

	if c.writer, err = must[cluster.Writer](clusterClient, "Writer"); err != nil {
		return c, err
	}
	if c.creator, err = must[cluster.Creator](clusterClient, "Creator"); err != nil {
		return c, err
	}
	if c.cloudInitReader, err = must[cluster.CloudInitReader](clusterClient, "CloudInitReader"); err != nil {
		return c, err
	}
	if c.snapshotReader, err = must[cluster.SnapshotReader](clusterClient, "SnapshotReader"); err != nil {
		return c, err
	}
	if c.snapshotWriter, err = must[cluster.SnapshotWriter](clusterClient, "SnapshotWriter"); err != nil {
		return c, err
	}
	if c.migrator, err = must[cluster.Migrator](clusterClient, "Migrator"); err != nil {
		return c, err
	}
	if c.consoleRelay, err = must[cluster.ConsoleRelay](clusterClient, "ConsoleRelay"); err != nil {
		return c, err
	}
	if c.serialRelay, err = must[cluster.TerminalRelay](clusterClient, "TerminalRelay"); err != nil {
		return c, err
	}
	if c.metricsReader, err = must[cluster.MetricsHistoryReader](clusterClient, "MetricsHistoryReader"); err != nil {
		return c, err
	}
	if c.metricsCurrentReader, err = must[cluster.MetricsCurrentReader](clusterClient, "MetricsCurrentReader"); err != nil {
		return c, err
	}
	if c.statusReader, err = must[cluster.VMStatusReader](clusterClient, "VMStatusReader"); err != nil {
		return c, err
	}
	if c.guestNetReader, err = must[cluster.GuestNetworkReader](clusterClient, "GuestNetworkReader"); err != nil {
		return c, err
	}

	return c, nil
}

// inventoryFreshness adapts inventory.Registry to httpapi.ClusterFreshnessChecker.
// It reads each cluster's Index.RefreshedAt (already maintained by the refresh
// goroutines) and the demoMode flag - zero cluster.Client calls from the health
// handler.
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
