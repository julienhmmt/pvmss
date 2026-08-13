//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"pvmss/server/internal/store"
	"testing"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// legacySchemaDDL is the exact v0.3 schema transcribed from
// backend/database/schema.go (not imported — constitution forbids
// server/ importing backend/). This fixture is the test-only source of
// truth for the legacy shape the recovery tool reads.
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

// openLegacyDB creates a temp-file SQLite database with the v0.3 schema
// and seeds it with the provided fixture data. The caller closes the DB
// via t.Cleanup.
func openLegacyDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(context.Background(), legacySchemaDDL); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	return db
}

// mustExec runs a statement against db, failing the test on error. It is the
// shared seed helper used by seedLegacyDB to keep each table's block flat.
func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()

	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// seedLegacyData populates every legacy table the recovery tool reads.
func seedLegacyDB(t *testing.T, db *sql.DB, seed legacySeed) {
	t.Helper()

	for _, n := range seed.Nodes {
		mustExec(t, db, `INSERT INTO enabled_nodes (name, enabled) VALUES (?, ?)`, n.name, n.enabled)
	}

	for _, s := range seed.Storages {
		mustExec(t, db, `INSERT INTO enabled_storages (storage_id, enabled) VALUES (?, ?)`, s.name, s.enabled)
	}

	for _, b := range seed.Bridges {
		mustExec(t, db, `INSERT INTO enabled_vmbrs (name, enabled) VALUES (?, ?)`, b.name, b.enabled)
	}

	for _, i := range seed.ISOs {
		mustExec(t, db, `INSERT INTO enabled_isos (name, enabled) VALUES (?, ?)`, i.name, i.enabled)
	}

	for _, tg := range seed.Tags {
		mustExec(t, db, `INSERT INTO tags (name) VALUES (?)`, tg)
	}

	for _, p := range seed.Profiles {
		mustExec(t, db, `INSERT INTO vm_profiles (id, name, description, config, enabled) VALUES (?, ?, ?, ?, ?)`,
			p.id, p.name, p.description, p.config, p.enabled)
	}

	if seed.VMLimits != nil {
		v := seed.VMLimits
		mustExec(t, db,
			`INSERT INTO vm_limits (id, max_vms, max_vm_per_user, max_network_cards, max_disk_per_vm, allow_custom_yaml, max_snapshots) VALUES (1, ?, ?, ?, ?, ?, ?)`,
			v.maxVMS, v.maxVMPerUser, v.maxNetworkCards, v.maxDiskPerVM, v.allowCustomYAML, v.maxSnapshots)
	}

	for _, n := range seed.NodeLimits {
		mustExec(t, db, `INSERT INTO node_limits (node_name, max_vms, max_vcpus, max_ram_gb, max_disk_gb) VALUES (?, ?, ?, ?, ?)`,
			n.node, n.maxVMs, n.maxVCPUs, n.maxRAMGB, n.maxDiskGB)
	}

	if seed.SFTPConfig {
		mustExec(t, db,
			`INSERT INTO sftp_config (id, enabled, host, port, username, private_key_path, remote_path) VALUES (1, 1, 'sftp.example.com', 22, 'user', '/key', '/snippets')`)
	}
}

// openV04DB creates a temp-file SQLite database and applies the v0.4
// schema migrations so the recovery tool has a valid target to write into.
func openV04DB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v04.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open v0.4 db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if err := storeRunMigrations(db); err != nil {
		t.Fatalf("run v0.4 migrations: %v", err)
	}

	return db
}

// storeRunMigrations applies the v0.4 schema migrations to a raw *sql.DB
// so the recovery tool has a valid target to write into during tests.
func storeRunMigrations(db *sql.DB) error {
	return store.RunMigrations(context.Background(), db, store.Migrations)
}

// --- Fixture seed structs ---

type legacySeed struct {
	Nodes []struct {
		name    string
		enabled bool
	}
	Storages []struct {
		name    string
		enabled bool
	}
	Bridges []struct {
		name    string
		enabled bool
	}
	ISOs []struct {
		name    string
		enabled bool
	}
	Tags       []string
	Profiles   []legacyProfile
	VMLimits   *legacyVMLimits
	NodeLimits []legacyNodeLimits
	SFTPConfig bool
}

type legacyProfile struct {
	id          string
	name        string
	description string
	config      string
	enabled     bool
}

type legacyVMLimits struct {
	maxVMS          int
	maxVMPerUser    int
	maxNetworkCards int
	maxDiskPerVM    int
	allowCustomYAML bool
	maxSnapshots    int
}

type legacyNodeLimits struct {
	node      string
	maxVMs    int
	maxVCPUs  int
	maxRAMGB  int
	maxDiskGB int
}

// defaultSeed returns a fixture with representative data across every
// table the recovery tool reads. Tests can override individual fields.
func defaultSeed() legacySeed {
	return legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{
			{name: "pve-a", enabled: true},
			{name: "pve-b", enabled: true},
		},
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
			{name: "nfs-share", enabled: true},
		},
		Bridges: []struct {
			name    string
			enabled bool
		}{
			{name: "vmbr0", enabled: true},
		},
		ISOs: []struct {
			name    string
			enabled bool
		}{
			{name: "local:iso/ubuntu-22.04.iso", enabled: true},
			{name: "nfs:iso/debian-12.iso", enabled: true},
		},
		Tags: []string{"pvmss", "prod", "dev"},
		Profiles: []legacyProfile{
			{
				id:      "small",
				name:    "Small",
				config:  `{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20,"disk_bus":"virtio"}`,
				enabled: true,
			},
			{
				id:      "large",
				name:    "Large",
				config:  `{"sockets":2,"cores":4,"ram_gb":8,"disk_gb":50,"disk_bus":"scsi"}`,
				enabled: true,
			},
		},
		VMLimits: &legacyVMLimits{
			maxVMS:          10,
			maxVMPerUser:    5,
			maxNetworkCards: 3,
			maxDiskPerVM:    20,
			allowCustomYAML: true,
			maxSnapshots:    8,
		},
		NodeLimits: []legacyNodeLimits{
			{node: "pve-a", maxVMs: 10, maxVCPUs: 32, maxRAMGB: 64, maxDiskGB: 500},
			{node: "pve-b", maxVMs: 5, maxVCPUs: 16, maxRAMGB: 32, maxDiskGB: 250},
		},
	}
}

// countRows is a test helper that returns the row count for a given SQL query.
func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}

	return count
}
