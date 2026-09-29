package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"pvmss/server/internal/config"
	"runtime/debug"
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

	logBanner(logger, cfg, []string{"default", "lab"}, bi)

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

	logBanner(slog.New(slog.NewJSONHandler(&buf, nil)), config.Configuration{}, nil, nil)

	if buf.Len() == 0 {
		t.Fatal("banner not logged without build info")
	}
}
