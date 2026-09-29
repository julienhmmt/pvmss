package main

import (
	"log/slog"
	"net"
	"runtime"
	"runtime/debug"
	"strconv"

	"pvmss/server/internal/config"
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
// otel and metricsAddr stay off/empty until tracing and /metrics land.
func logBanner(logger *slog.Logger, cfg config.Configuration, clusters []string, bi *debug.BuildInfo) {
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
		"otel", false,
		"metricsAddr", "",
	)
}
