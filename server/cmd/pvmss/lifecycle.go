package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"

	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/telemetry"
)

// listenAddr is the address the HTTP server binds, shared by the banner and
// the listener so they can never disagree.
func listenAddr(cfg config.Configuration) string {
	return net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
}

// vcsSetting reads one VCS stamp (vcs.revision, vcs.time) from the build info;
// empty when the binary was built without VCS stamping (go run, -buildvcs=false).
func vcsSetting(bi *debug.BuildInfo, key string) string {
	if bi == nil {
		return ""
	}

	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}

	return ""
}

// logBanner emits the single "pvmss starting" line: what is running and how it
// is configured, so a log excerpt alone answers "which build, against what".
// otelHost is the OTLP endpoint host only (never headers or credentials);
// empty means tracing is off. metricsAddr is empty when PVMSS_METRICS_PORT is unset.
func logBanner(logger *slog.Logger, cfg config.Configuration, clusters []string, bi *debug.BuildInfo, otelHost string) {
	metricsAddr := ""
	if cfg.MetricsPort != 0 {
		metricsAddr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.MetricsPort))
	}

	goVersion := runtime.Version()
	if bi != nil && bi.GoVersion != "" {
		goVersion = bi.GoVersion
	}

	logger.Info("pvmss starting", "component", "main",
		"version", appVersion,
		"commit", vcsSetting(bi, "vcs.revision"),
		"commitTime", vcsSetting(bi, "vcs.time"),
		"goVersion", goVersion,
		"clusterSource", cfg.ClusterSource,
		"clusters", clusters,
		"logLevel", cfg.LogLevel,
		"logFormat", cfg.LogFormat,
		"addr", listenAddr(cfg),
		"otel", otelHost != "",
		"otelEndpoint", otelHost,
		"metricsAddr", metricsAddr,
	)
}

// startMetricsServer serves handler at GET /metrics on its own listener
// (PVMSS_HOST:PVMSS_METRICS_PORT). It is a no-op when the port is 0. Only that
// one route exists: the endpoint has no auth, so it must never share the main
// port or grow other routes. The returned stop drains it with the main server.
func startMetricsServer(cfg config.Configuration, handler http.Handler, logger *slog.Logger) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if cfg.MetricsPort == 0 || handler == nil {
		return noop, nil
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", handler)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.MetricsPort))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: readHeaderTimeout, WriteTimeout: writeTimeout}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return noop, fmt.Errorf("listen for metrics on %s: %w", addr, err)
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server error", "component", "main", "addr", addr, "error", err)
		}
	}()

	logger.Info("metrics endpoint ready", "component", "main", "addr", addr)

	return srv.Shutdown, nil
}

// inventoryStats adapts the inventory registry to the observable gauges: last
// successful refresh and VM counts by status, per cluster, read at scrape time.
func inventoryStats(reg *inventory.Registry) func() []telemetry.ClusterStat {
	return func() []telemetry.ClusterStat {
		all := reg.All()
		stats := make([]telemetry.ClusterStat, 0, len(all))

		for name, idx := range all {
			if idx == nil {
				continue
			}

			byStatus := make(map[string]int)
			for _, v := range idx.ByVMID {
				byStatus[string(v.Status)]++
			}

			stats = append(stats, telemetry.ClusterStat{Cluster: name, LastSuccess: idx.RefreshedAt, VMsByStatus: byStatus})
		}

		return stats
	}
}
