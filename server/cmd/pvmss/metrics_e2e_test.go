package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func get(t *testing.T, url string) (int, string) {
	t.Helper()

	resp, err := http.Get(url) //nolint:noctx,gosec // local test server
	if err != nil {
		return 0, err.Error()
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(body)
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()

	for start := time.Now(); time.Since(start) < 5*time.Second; time.Sleep(50 * time.Millisecond) {
		if ok() {
			return
		}
	}

	t.Fatalf("timed out waiting for %s", what)
}

// PVMSS_METRICS_PORT set: /metrics on that port lists the v1 instruments after
// one request and the startup refresh, with no vmid/user/node label, while the
// main port answers /metrics with 404 (not the SPA shell).
//

func TestRun_MetricsEndpoint(t *testing.T) {
	prevMP, prevTP := otel.GetMeterProvider(), otel.GetTracerProvider()

	t.Cleanup(func() { otel.SetMeterProvider(prevMP); otel.SetTracerProvider(prevTP) })

	mainPort, metricsPort := freePort(t), freePort(t)
	dir := t.TempDir()
	webDir := filepath.Join(dir, "web")

	if err := os.MkdirAll(webDir, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><html>SPA-SHELL</html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PVMSS_PORT", strconv.Itoa(mainPort))
	t.Setenv("PVMSS_METRICS_PORT", strconv.Itoa(metricsPort))
	t.Setenv("PVMSS_DB_PATH", filepath.Join(dir, "pvmss.db"))
	t.Setenv("PVMSS_WEB_DIR", webDir)
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", "stdout")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("PVMSS_CLUSTER_SOURCE", "fake")

	done := make(chan int, 1)

	go func() { done <- run() }()

	mainURL := "http://127.0.0.1:" + strconv.Itoa(mainPort)
	metricsURL := "http://127.0.0.1:" + strconv.Itoa(metricsPort) + "/metrics"

	waitFor(t, "main server", func() bool { c, _ := get(t, mainURL+"/health"); return c == http.StatusOK })

	if c, _ := get(t, mainURL+"/api/v1/public/version"); c != http.StatusOK {
		t.Fatalf("version = %d", c)
	}

	if c, body := get(t, mainURL+"/metrics"); c != http.StatusNotFound || strings.Contains(body, "SPA-SHELL") {
		t.Fatalf("main port /metrics = %d, want 404 and no SPA shell", c)
	}

	var body string

	waitFor(t, "metrics with refresh data", func() bool {
		_, body = get(t, metricsURL)
		return strings.Contains(body, "pvmss_inventory_last_success") && strings.Contains(body, "http_server_request_duration")
	})

	for _, want := range []string{
		"http_server_request_duration_seconds", "pvmss_inventory_refresh_duration_seconds",
		"pvmss_inventory_last_success_seconds", "pvmss_vms", "go_",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape lacks %q", want)
		}
	}

	for _, forbidden := range []string{"vmid=", "user=", "node="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("scrape carries forbidden label %q", forbidden)
		}
	}

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("run() = %d", code)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("run() did not stop")
	}

	if c, _ := get(t, metricsURL); c != 0 {
		t.Errorf("metrics listener still up after shutdown (status %d)", c)
	}
}
