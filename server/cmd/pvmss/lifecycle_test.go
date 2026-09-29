package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"pvmss/server/internal/config"
	"runtime/debug"
	"strconv"
	"testing"
)

func TestLogBanner_ContainsEveryField(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	cfg := config.Configuration{Host: "0.0.0.0", Port: 50000, ClusterSource: "fake", LogLevel: "info", LogFormat: "json"}
	bi := &debug.BuildInfo{GoVersion: "go1.27.1", Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.time", Value: "2026-09-29T10:00:00Z"},
	}}

	logBanner(logger, cfg, []string{"default", "lab"}, bi, "")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("banner is not one JSON line: %v: %s", err, buf.String())
	}

	want := map[string]any{
		"msg": "pvmss starting", "level": "INFO", "version": appVersion, "commit": "abc123",
		"commitTime": "2026-09-29T10:00:00Z", "goVersion": "go1.27.1", "clusterSource": "fake",
		"logLevel": "info", "logFormat": "json", "addr": "0.0.0.0:50000", "otel": false, "metricsAddr": "",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}

	if names, ok := got["clusters"].([]any); !ok || len(names) != 2 {
		t.Errorf("clusters = %v", got["clusters"])
	}
}

func TestLogBanner_NilBuildInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logBanner(slog.New(slog.NewJSONHandler(&buf, nil)), config.Configuration{}, nil, nil, "")

	if buf.Len() == 0 {
		t.Fatal("banner not logged without build info")
	}
}

func TestLogBanner_OTelEnabledShowsHostOnly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logBanner(slog.New(slog.NewJSONHandler(&buf, nil)), config.Configuration{}, nil, nil, "otel.example:4318")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got["otel"] != true || got["otelEndpoint"] != "otel.example:4318" {
		t.Errorf("otel fields = %v / %v", got["otel"], got["otelEndpoint"])
	}
}

func freePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address %T", ln.Addr())
	}

	return addr.Port
}

func TestStartMetricsServer_ServesOnlyGetMetrics(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	cfg := config.Configuration{Host: "127.0.0.1", MetricsPort: port}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("metrics-body")) })

	stop, err := startMetricsServer(cfg, handler, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startMetricsServer: %v", err)
	}

	defer func() { _ = stop(context.Background()) }()

	base := "http://127.0.0.1:" + strconv.Itoa(port)

	for path, want := range map[string]int{"/metrics": http.StatusOK, "/": http.StatusNotFound, "/health": http.StatusNotFound} {
		resp, err := http.Get(base + path) //nolint:noctx,gosec // local test server
		if err != nil {
			t.Fatal(err)
		}

		_ = resp.Body.Close()

		if resp.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", path, resp.StatusCode, want)
		}
	}

	resp, err := http.Post(base+"/metrics", "text/plain", nil) //nolint:noctx,gosec // local test server
	if err != nil {
		t.Fatal(err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /metrics = %d, want 405", resp.StatusCode)
	}
}

func TestStartMetricsServer_DisabledBindsNothing(t *testing.T) {
	t.Parallel()

	stop, err := startMetricsServer(config.Configuration{Host: "127.0.0.1"}, http.NotFoundHandler(), slog.New(slog.DiscardHandler))
	if err != nil || stop == nil {
		t.Fatalf("stop=%v err=%v", stop != nil, err)
	}

	if err := stop(context.Background()); err != nil {
		t.Fatalf("no-op stop: %v", err)
	}
}
