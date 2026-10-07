package main

import (
	"context"
	"database/sql"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

const (
	recoverCmd      = "pvmss-recover"
	legacyDBFlag    = "--legacy-db"
	v04DBFlag       = "--v0.4-db"
	clusterNameFlag = "--cluster-name"
	dryRunFlag      = "--dry-run"
	testClusterName = "test-cluster"
)

// legacySchemaDDL is the v0.3 schema the recovery tool reads, transcribed
// from internal/recovery/fixture_test.go so this white-box cmd test can
// build a legacy fixture without importing the recovery_test package.
const legacySchemaDDL = `
CREATE TABLE IF NOT EXISTS enabled_nodes (
    name       TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS enabled_storages (
    storage_id TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS enabled_vmbrs (
    name       TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS enabled_isos (
    name       TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tags (
    name       TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS vm_profiles (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    config      TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT 1,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS vm_limits (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    max_vms           INTEGER NOT NULL DEFAULT 10,
    max_vm_per_user   INTEGER NOT NULL DEFAULT 2,
    max_network_cards INTEGER NOT NULL DEFAULT 2,
    max_disk_per_vm   INTEGER NOT NULL DEFAULT 4,
    allow_custom_yaml BOOLEAN NOT NULL DEFAULT 0,
    max_snapshots     INTEGER NOT NULL DEFAULT 3,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS node_limits (
    node_name  TEXT PRIMARY KEY,
    max_vms    INTEGER,
    max_vcpus  INTEGER,
    max_ram_gb INTEGER,
    max_disk_gb INTEGER,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sftp_config (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    enabled          BOOLEAN NOT NULL DEFAULT 0,
    host             TEXT,
    port             INTEGER DEFAULT 22,
    username         TEXT,
    private_key_path TEXT,
    remote_path      TEXT,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

// buildLegacyFixture creates a file-backed legacy SQLite database with the
// v0.3 schema and a small representative dataset, then returns its path.
func buildLegacyFixture(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "legacy.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, legacySchemaDDL); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	exec := func(q string, args ...any) {
		t.Helper()

		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}

	exec(`INSERT INTO enabled_nodes (name, enabled) VALUES ('pve-a', 1), ('pve-b', 1)`)
	exec(`INSERT INTO enabled_storages (storage_id, enabled) VALUES ('local-lvm', 1)`)
	exec(`INSERT INTO enabled_vmbrs (name, enabled) VALUES ('vmbr0', 1)`)
	exec(`INSERT INTO enabled_isos (name, enabled) VALUES ('local:iso/ubuntu-22.04.iso', 1)`)
	exec(`INSERT INTO tags (name) VALUES ('prod'), ('dev')`)
	exec(`INSERT INTO vm_profiles (id, name, config, enabled) VALUES ('small', 'Small', '{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20,"disk_bus":"virtio"}', 1)`)
	exec(`INSERT INTO vm_limits (id, max_vms, max_vm_per_user, max_network_cards, max_disk_per_vm, allow_custom_yaml, max_snapshots) VALUES (1, 10, 5, 3, 20, 1, 8)`)
	exec(`INSERT INTO node_limits (node_name, max_vms, max_vcpus, max_ram_gb, max_disk_gb) VALUES ('pve-a', 10, 32, 64, 500)`)

	if err := db.Close(); err != nil {
		t.Fatalf("close legacy fixture: %v", err)
	}

	return path
}

// resetFlags replaces the global flag.CommandLine with a fresh FlagSet so
// run() can re-register and re-parse its flags on every call. It also
// captures and restores os.Args and stdout. The returned restore function
// must be called before reading the captured stdout via the second return
// value.
func resetFlags(t *testing.T, args []string) (restore func(), stdout func() string) {
	t.Helper()

	savedCommandLine := flag.CommandLine
	savedArgs := os.Args
	savedStdout := os.Stdout

	flag.CommandLine = flag.NewFlagSet("pvmss-recover-test", flag.ContinueOnError)
	os.Args = args

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	os.Stdout = w

	return func() {
			os.Stdout = savedStdout
			_ = w.Close()
			flag.CommandLine = savedCommandLine
			os.Args = savedArgs
		}, func() string {
			out, _ := io.ReadAll(r)
			_ = r.Close()

			return string(out)
		}
}

func TestOpenSQLite_EmptyPath(t *testing.T) {
	t.Parallel()

	if _, err := openSQLite("", false); err == nil {
		t.Fatalf("expected error for empty path, got nil")
	}
}

func TestOpenSQLite_ReadOnly_NonExistent(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	if _, err := openSQLite(missing, true); err == nil {
		t.Fatalf("expected error for read-only non-existent db, got nil")
	}
}

func TestOpenSQLite_ReadWrite_CreatesNew(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fresh.db")

	db, err := openSQLite(path, false)
	if err != nil {
		t.Fatalf("openSQLite read-write: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file to be created: %v", err)
	}
}

func TestOpenSQLite_ReadOnly_Existing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "existing.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE t (c INTEGER)`); err != nil {
		t.Fatalf("seed table: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ro, err := openSQLite(path, true)
	if err != nil {
		t.Fatalf("openSQLite read-only on existing db: %v", err)
	}

	if _, err := ro.ExecContext(context.Background(), `INSERT INTO t (c) VALUES (1)`); err == nil {
		t.Fatalf("expected write to fail on read-only connection")
	}

	_ = ro.Close()
}

//nolint:paralleltest // serial: mutates global flag.CommandLine, os.Args and os.Stdout
func TestRun_MissingFlags_Returns1(t *testing.T) {
	restore, _ := resetFlags(t, []string{recoverCmd})
	defer restore()

	if code := run(); code != 1 {
		t.Fatalf("run() with no flags returned %d, want 1", code)
	}
}

//nolint:paralleltest // serial: mutates global flag.CommandLine, os.Args and os.Stdout
func TestRun_InvalidClusterName_Returns2(t *testing.T) {
	legacyPath := buildLegacyFixture(t)
	v04Path := filepath.Join(t.TempDir(), "v04.db")

	restore, _ := resetFlags(t, []string{
		recoverCmd,
		legacyDBFlag, legacyPath,
		v04DBFlag, v04Path,
		clusterNameFlag, "Not Valid!",
	})
	defer restore()

	if code := run(); code != 2 {
		t.Fatalf("run() with invalid cluster name returned %d, want 2", code)
	}
}

//nolint:paralleltest // serial: mutates global flag.CommandLine, os.Args and os.Stdout
func TestRun_LegacyDBNotFound_Returns1(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	v04Path := filepath.Join(t.TempDir(), "v04.db")

	restore, _ := resetFlags(t, []string{
		recoverCmd,
		legacyDBFlag, missing,
		v04DBFlag, v04Path,
		clusterNameFlag, testClusterName,
	})
	defer restore()

	if code := run(); code != 1 {
		t.Fatalf("run() with missing legacy db returned %d, want 1", code)
	}
}

//nolint:paralleltest // serial: mutates global flag.CommandLine, os.Args and os.Stdout
func TestRun_DryRun_Success(t *testing.T) {
	legacyPath := buildLegacyFixture(t)
	v04Path := filepath.Join(t.TempDir(), "v04.db")

	restore, _ := resetFlags(t, []string{
		recoverCmd,
		legacyDBFlag, legacyPath,
		v04DBFlag, v04Path,
		clusterNameFlag, testClusterName,
		"--session-secret", "test-session-secret-at-least-32-bytes!!",
		dryRunFlag,
	})
	defer restore()

	code := run()
	if code != 0 {
		t.Fatalf("run() dry-run returned %d, want 0", code)
	}
}

//nolint:paralleltest // serial: mutates global flag.CommandLine, os.Args and os.Stdout
func TestRun_DryRun_StorageSkippedWithoutCreds(t *testing.T) {
	legacyPath := buildLegacyFixture(t)
	v04Path := filepath.Join(t.TempDir(), "v04.db")

	restore, stdout := resetFlags(t, []string{
		recoverCmd,
		legacyDBFlag, legacyPath,
		v04DBFlag, v04Path,
		clusterNameFlag, testClusterName,
		"--session-secret", "test-session-secret-at-least-32-bytes!!",
		dryRunFlag,
	})

	code := run()

	restore()

	output := stdout()

	if code != 0 {
		t.Fatalf("run() dry-run returned %d, want 0", code)
	}

	if !strings.Contains(output, "SUMMARY: written=") {
		t.Errorf("output missing SUMMARY line:\n%s", output)
	}

	if !strings.Contains(output, "no Proxmox credentials") {
		t.Errorf("output should mention skipped storage without creds:\n%s", output)
	}
}

func TestRun_SessionSecretFromEnv(t *testing.T) {
	legacyPath := buildLegacyFixture(t)
	v04Path := filepath.Join(t.TempDir(), "v04.db")

	t.Setenv("SESSION_SECRET", "env-session-secret-at-least-32-bytes!!")

	restore, _ := resetFlags(t, []string{
		recoverCmd,
		legacyDBFlag, legacyPath,
		v04DBFlag, v04Path,
		clusterNameFlag, testClusterName,
		dryRunFlag,
	})
	defer restore()

	if code := run(); code != 0 {
		t.Fatalf("run() with env SESSION_SECRET returned %d, want 0", code)
	}
}

//nolint:paralleltest // serial: mutates global flag.CommandLine, os.Args and os.Stdout
func TestRun_ProxmoxCredsFromFlags(t *testing.T) {
	legacyPath := buildLegacyFixture(t)
	v04Path := filepath.Join(t.TempDir(), "v04.db")

	restore, stdout := resetFlags(t, []string{
		recoverCmd,
		legacyDBFlag, legacyPath,
		v04DBFlag, v04Path,
		clusterNameFlag, testClusterName,
		"--session-secret", "test-session-secret-at-least-32-bytes!!",
		"--proxmox-url", "https://pve.example.com:8006/api2/json",
		"--proxmox-token-id", "pve-test-token",
		dryRunFlag,
	})

	code := run()

	restore()

	output := stdout()

	if code != 0 {
		t.Fatalf("run() with proxmox flag creds returned %d, want 0", code)
	}

	if !strings.Contains(output, "clusters        1 written") {
		t.Errorf("output should report 1 cluster written:\n%s", output)
	}
}
