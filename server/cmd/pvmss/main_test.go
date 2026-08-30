//nolint:noctx // test scaffolding does not need real context
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// displayNameFake embeds cluster.Fake but overrides DisplayName, so it is not
// itself a cluster.Fake: the concrete type assertion in
// discoverClusterDisplayNames sees a non-fake client and attempts discovery.
type displayNameFake struct {
	cluster.Fake
	displayName string
	err         error
}

func (f displayNameFake) DisplayName(context.Context) (string, error) {
	return f.displayName, f.err
}

func TestResolveWebBuildDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "web", "build"), 0o750); err != nil {
		t.Fatalf("create web build dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	t.Setenv("PVMSS_WEB_DIR", dir)

	got, err := resolveWebBuildDir(dir)
	if err != nil {
		t.Fatalf("resolveWebBuildDir: %v", err)
	}

	if got != dir {
		t.Fatalf("resolved path = %q, want %q", got, dir)
	}
}

//nolint:paralleltest // serial: process and listener lifecycle tests
func TestResolveWebBuildDir_Invalid(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := resolveWebBuildDir(filePath); err == nil {
		t.Fatalf("expected error for file path, got nil")
	}

	if _, err := resolveWebBuildDir(filepath.Join(dir, "missing")); err == nil {
		t.Fatalf("expected error for missing path, got nil")
	}
}

//nolint:paralleltest // serial: process and listener lifecycle tests
func TestResolveWebBuildDir_Fallback(t *testing.T) {
	dir := t.TempDir()

	build := filepath.Join(dir, "web", "build")
	if err := os.MkdirAll(build, 0o750); err != nil {
		t.Fatalf("create web build dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(build, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	t.Chdir(dir)

	got, err := resolveWebBuildDir("")
	if err != nil {
		t.Fatalf("resolveWebBuildDir: %v", err)
	}

	if got != "web/build" {
		t.Fatalf("resolved path = %q, want %q", got, "web/build")
	}
}

//nolint:paralleltest // serial: process and listener lifecycle tests
func TestValidateWebBuildDir(t *testing.T) {
	dir := t.TempDir()

	build := filepath.Join(dir, "build")
	if err := os.MkdirAll(build, 0o750); err != nil {
		t.Fatalf("create build dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(build, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	if err := validateWebBuildDir(build); err != nil {
		t.Fatalf("validate build dir: %v", err)
	}

	notADir := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := validateWebBuildDir(notADir); err == nil {
		t.Fatalf("expected error for file path, got nil")
	}

	if err := validateWebBuildDir(filepath.Join(dir, "missing")); err == nil {
		t.Fatalf("expected error for missing path, got nil")
	}
}

//nolint:paralleltest // serial: process and listener lifecycle tests
func TestRun_InvalidConfig_ExitsWithoutListening(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "pvmss")

	build := exec.Command("go", "build", "-o", bin, ".") //nolint:gosec // building test binary from trusted source
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is not *net.TCPAddr: %T", ln.Addr())
	}

	port := tcpAddr.Port
	_ = ln.Close()

	dbPath := filepath.Join(t.TempDir(), "pvmss.db")
	env := []string{
		fmt.Sprintf("PVMSS_PORT=%d", port),
		"PVMSS_DB_PATH=" + dbPath,
		"LOG_LEVEL=invalid",
		"LOG_FORMAT=json",
		"LOG_OUTPUT=stdout",
	}

	cmd := exec.Command(bin) //nolint:gosec // running test binary built from trusted source

	cmd.Env = append(os.Environ(), env...)

	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected non-zero exit, got 0; output: %s", out)
	}

	if elapsed > 2*time.Second {
		t.Fatalf("process took %v to exit, want under 2s", elapsed)
	}

	if !strings.Contains(string(out), "LOG_LEVEL") {
		t.Fatalf("expected output to mention LOG_LEVEL, got: %s", out)
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	if conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
		_ = conn.Close()

		t.Fatalf("server should not have accepted a connection on %s", addr)
	}
}

func TestRun_InvalidConfig_Returns1(t *testing.T) {
	t.Setenv("PVMSS_PORT", "52003")
	t.Setenv("PVMSS_DB_PATH", filepath.Join(t.TempDir(), "pvmss.db"))
	t.Setenv("LOG_LEVEL", "invalid")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", "stdout")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("PVMSS_CLUSTER_SOURCE", "fake")

	if code := run(); code != 1 {
		t.Fatalf("run() returned %d, want 1", code)
	}
}

func TestRun_WebDirNotFound_Returns1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PVMSS_PORT", "52005")
	t.Setenv("PVMSS_DB_PATH", filepath.Join(dir, "pvmss.db"))
	t.Setenv("PVMSS_WEB_DIR", filepath.Join(dir, "missing"))
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", "stdout")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("PVMSS_CLUSTER_SOURCE", "fake")

	if code := run(); code != 1 {
		t.Fatalf("run() returned %d, want 1", code)
	}
}

func TestRun_LoggerCreationFails_Returns1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PVMSS_PORT", "52006")
	t.Setenv("PVMSS_DB_PATH", filepath.Join(dir, "pvmss.db"))
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", filepath.Join(dir, "missing", "log.txt"))

	if code := run(); code != 1 {
		t.Fatalf("run() returned %d, want 1", code)
	}
}

func TestRun_DatabaseOpenFails_Returns1(t *testing.T) {
	dir := t.TempDir()

	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	t.Setenv("PVMSS_PORT", "52004")
	t.Setenv("PVMSS_DB_PATH", filepath.Join(blocked, "pvmss.db"))
	t.Setenv("PVMSS_WEB_DIR", dir)
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", "stdout")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("PVMSS_CLUSTER_SOURCE", "fake")

	if code := run(); code != 1 {
		t.Fatalf("run() returned %d, want 1", code)
	}
}

func TestRun(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is not *net.TCPAddr: %T", ln.Addr())
	}

	port := tcpAddr.Port
	_ = ln.Close()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "pvmss.db")

	webDir := filepath.Join(dir, "web")
	if err := os.MkdirAll(webDir, 0o750); err != nil {
		t.Fatalf("create web dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><html></html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	t.Setenv("PVMSS_PORT", strconv.Itoa(port))
	t.Setenv("PVMSS_DB_PATH", dbPath)
	t.Setenv("PVMSS_WEB_DIR", webDir)
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", "stdout")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("PVMSS_CLUSTER_SOURCE", "fake")

	done := make(chan int, 1)
	go func() { done <- run() }()

	var healthOK bool

	for start := time.Now(); time.Since(start) < 2*time.Second; time.Sleep(50 * time.Millisecond) {
		resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
		if err == nil {
			_ = resp.Body.Close()

			healthOK = resp.StatusCode == http.StatusOK
			if healthOK {
				break
			}
		}
	}

	if !healthOK {
		t.Fatalf("server did not become healthy")
	}

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("run() returned %d, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("run() did not stop after signal")
	}
}

// openTestStore opens a store in a temp dir with the fake cluster source so
// ensureSeedClusters populates the demo cluster rows.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Configuration{
		DBPath:        filepath.Join(dir, "pvmss.db"),
		SessionSecret: strings.Repeat("s", 32),
		ClusterSource: "fake",
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// TestDiscoverClusterDisplayNames_PopulatesEmptyRows — a non-fake cluster
// client that returns a display name has it persisted at startup. Fake
// clusters are skipped: their DisplayName() would just echo the internal name.
//
//nolint:paralleltest // serial: shared database state
func TestDiscoverClusterDisplayNames_PopulatesEmptyRows(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	//nolint:gosec // test fixture credential
	//nolint:gosec // test fixture credential
	if err := st.CreateCluster(ctx, store.ClusterRow{Name: "proxmox", URL: "https://pve.example.com:8006/api2/json", TokenID: "pve-test-token", TokenSecret: "pve-test-secret"}); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}

	factory := func(store.ClusterRow) (cluster.Client, error) {
		return displayNameFake{displayName: "Production PVE"}, nil
	}

	registry, err := cluster.NewRegistryWithFactory(factory, rows)
	if err != nil {
		t.Fatalf("NewRegistryWithFactory: %v", err)
	}

	// Wipe display names so discovery has work to do.
	for _, row := range rows {
		if err := st.SetClusterDisplayName(ctx, row.Name, ""); err != nil {
			t.Fatalf("clear DisplayName %q: %v", row.Name, err)
		}
	}

	clearedRows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters after clear: %v", err)
	}

	_ = clearedRows // kept for clarity: discovery now reads rows internally

	discoverClusterDisplayNames(ctx, registry, st, slog.Default())

	row, err := st.GetCluster(ctx, "proxmox")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}

	if row.DisplayName != "Production PVE" {
		t.Fatalf("DisplayName = %q, want %q", row.DisplayName, "Production PVE")
	}
}

// TestDiscoverClusterDisplayNames_SkipsFake — fake clusters are never
// discovered: their DisplayName() just returns the logical cluster name, which
// would re-introduce the "default" label. Fake display names come from the
// seed instead.
//
//nolint:paralleltest // serial: shared fake cluster registry state
func TestDiscoverClusterDisplayNames_SkipsFake(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}

	// Wipe seeded display names to simulate a database created before the seed
	// fix — discovery must not fill the fake rows with their internal names.
	for _, row := range rows {
		if err := st.SetClusterDisplayName(ctx, row.Name, ""); err != nil {
			t.Fatalf("clear DisplayName %q: %v", row.Name, err)
		}
	}

	clearedRows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters after clear: %v", err)
	}

	registry, err := cluster.NewRegistry("fake", clearedRows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	discoverClusterDisplayNames(ctx, registry, st, slog.Default())

	refreshed, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters after discovery: %v", err)
	}

	for _, row := range refreshed {
		if row.DisplayName != "" {
			t.Errorf("fake cluster %q got DisplayName %q, want empty", row.Name, row.DisplayName)
		}
	}
}

// TestDiscoverClusterDisplayNames_PreservesExisting — a cluster that already
// has a display name is not overwritten by startup discovery.
//
//nolint:paralleltest // serial: shared database state
func TestDiscoverClusterDisplayNames_PreservesExisting(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	//nolint:gosec // test fixture credential
	if err := st.CreateCluster(ctx, store.ClusterRow{Name: "proxmox", URL: "https://pve.example.com:8006/api2/json", TokenID: "pve-test-token", TokenSecret: "pve-test-secret"}); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	const customName = "admin-set-name"
	if err := st.SetClusterDisplayName(ctx, "proxmox", customName); err != nil {
		t.Fatalf("SetClusterDisplayName: %v", err)
	}

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}

	factory := func(store.ClusterRow) (cluster.Client, error) {
		return displayNameFake{displayName: "Production PVE"}, nil
	}

	registry, err := cluster.NewRegistryWithFactory(factory, rows)
	if err != nil {
		t.Fatalf("NewRegistryWithFactory: %v", err)
	}

	discoverClusterDisplayNames(ctx, registry, st, slog.Default())

	row, err := st.GetCluster(ctx, "proxmox")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}

	if row.DisplayName != customName {
		t.Fatalf("DisplayName = %q, want %q (existing value should be preserved)", row.DisplayName, customName)
	}
}
