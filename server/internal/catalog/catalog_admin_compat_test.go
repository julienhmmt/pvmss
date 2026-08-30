package catalog_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/store"
	"testing"

	_ "modernc.org/sqlite"
)

// openRawDB opens a raw SQLite database in a temp dir without running any
// migrations — the compatibility test needs to control exactly which versions
// are applied so it can capture outputs at V7 and again after V9.
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "compat.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

// runMigrationsUpTo applies store.Migrations up to and including maxVersion.
func runMigrationsUpTo(t *testing.T, db *sql.DB, maxVersion int) {
	t.Helper()

	var subset []store.Migration

	for _, m := range store.Migrations {
		if m.Version > maxVersion {
			break
		}

		subset = append(subset, m)
	}

	if err := store.RunMigrations(context.Background(), db, subset); err != nil {
		t.Fatalf("RunMigrations up to V%d: %v", maxVersion, err)
	}
}

// columnExists checks whether a column exists on a table after all migrations
// have run. Used to prove V9's ALTER TABLE landed.
func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatalf("pragma table_info %s: %v", table, err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma row: %v", err)
		}

		if name == column {
			return true
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma rows: %v", err)
	}

	return false
}

// rawNode is the pre-T11 shape captured at V7 using direct SQL (no enabled
// column exists yet). It matches catalog.Node exactly.
type rawNode struct {
	Name string
}

// rawStorage matches catalog.Storage.
type rawStorage struct {
	Name string
	Node string
}

type rawBridge struct {
	Name string
	Node string
}

// rawISO matches catalog.ISO.
type rawISO struct {
	Storage string
	File    string
}

// rawProfile matches catalog.Profile.
type rawProfile struct {
	ID       string
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// captureAtV7 reads the five T06/T07 row sets using direct SQL — the queries
// T06/T07 used before T11 added the enabled column. This is the "before"
// snapshot: what the functions returned under V7.
func captureAtV7(t *testing.T, db *sql.DB) (
	[]rawNode,
	[]rawStorage,
	[]rawBridge,
	[]rawISO,
	[]rawProfile,
) {
	t.Helper()

	ctx := context.Background()

	nodes := queryRows(ctx, t, db, "nodes at V7",
		`SELECT name FROM catalog_nodes WHERE cluster = 'default' ORDER BY name`,
		func() rawNode { return rawNode{} },
		func(r *rawNode, rows *sql.Rows) error { return rows.Scan(&r.Name) },
	)

	storages := queryRows(ctx, t, db, "storages at V7",
		`SELECT name, node FROM catalog_storages WHERE cluster = 'default' ORDER BY node, name`,
		func() rawStorage { return rawStorage{} },
		func(r *rawStorage, rows *sql.Rows) error { return rows.Scan(&r.Name, &r.Node) },
	)

	bridges := queryRows(ctx, t, db, "bridges at V7",
		`SELECT name, node FROM catalog_bridges WHERE cluster = 'default' ORDER BY node, name`,
		func() rawBridge { return rawBridge{} },
		func(r *rawBridge, rows *sql.Rows) error { return rows.Scan(&r.Name, &r.Node) },
	)

	isos := queryRows(ctx, t, db, "isos at V7",
		`SELECT storage, file FROM catalog_isos WHERE cluster = 'default' ORDER BY file`,
		func() rawISO { return rawISO{} },
		func(r *rawISO, rows *sql.Rows) error { return rows.Scan(&r.Storage, &r.File) },
	)

	profiles := queryRows(ctx, t, db, "profiles at V7",
		`SELECT id, label, cpu_cores, memory_mb, disk_gb, bus FROM catalog_profiles WHERE cluster = 'default' ORDER BY id`,
		func() rawProfile { return rawProfile{} },
		func(r *rawProfile, rows *sql.Rows) error {
			return rows.Scan(&r.ID, &r.Label, &r.CPUCores, &r.MemoryMB, &r.DiskGB, &r.Bus)
		},
	)

	return nodes, storages, bridges, isos, profiles
}

// queryRows is a generic helper that runs a query, scans each row into a new T
// via the scan function, and returns the collected slice. It handles
// rows.Err() and closes the rows. Used by captureAtV7 to avoid 5× duplicated
// query-scan-collect boilerplate.
func queryRows[T any](
	ctx context.Context,
	t *testing.T,
	db *sql.DB,
	label string,
	query string,
	newRow func() T,
	scan func(*T, *sql.Rows) error,
) []T {
	t.Helper()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("query %s: %v", label, err)
	}

	defer func() { _ = rows.Close() }()

	var out []T

	for rows.Next() {
		r := newRow()

		if err := scan(&r, rows); err != nil {
			t.Fatalf("scan %s: %v", label, err)
		}

		out = append(out, r)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s rows: %v", label, err)
	}

	return out
}

// captureAtLatest reads the five T06/T07 row sets through the actual catalog
// functions (which now carry AND enabled = 1). This is the "after" snapshot:
// what the functions return under V9 with zero toggles performed. The caller
// must have added the sockets column (V21's DDL) manually so catalog.Profiles
// can run — the test stops at V9 because V14 drops catalog_bridges.
func captureAtLatest(t *testing.T, st *store.Store) (
	[]catalog.Node,
	[]catalog.Storage,
	[]catalog.Bridge,
	[]catalog.ISO,
	[]catalog.Profile,
) {
	t.Helper()

	ctx := context.Background()

	resources, err := catalog.ApprovedResources(ctx, st, "default")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	profiles, err := catalog.Profiles(ctx, st, "default")
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}

	return resources.Nodes, resources.Storages, resources.Bridges, resources.ISOs, profiles
}

// TestCatalogAdminCompat_RowSetsIdenticalBeforeAndAfterV9 is SC-003: the
// mechanical proof that T11's migration (V9: enabled column + catalog_tags)
// does not change what T06's and T07's five existing read functions return.
// The fixture DB is built at V7, outputs captured via direct SQL (the pre-T11
// query shapes), then migrated forward to V9 with zero admin toggles
// performed, and outputs captured again through the actual catalog functions
// (which now carry AND enabled = 1). The two snapshots must be identical —
// every existing row's enabled defaults to 1.
//
//nolint:paralleltest // serial: shared database fixture
func TestCatalogAdminCompat_RowSetsIdenticalBeforeAndAfterV9(t *testing.T) {
	db := openRawDB(t)

	// Build at V7 (T06's seed).
	runMigrationsUpTo(t, db, 7)

	beforeNodes, beforeStorages, beforeBridges, beforeISOs, beforeProfiles := captureAtV7(t, db)

	// Migrate forward through V9, assert V9's changes landed.
	runMigrationsUpTo(t, db, 9)

	assertV9EnabledColumnOnAllTables(t, db)
	assertPvmssTagSeeded(t, db)

	// Add the sockets column manually (same DDL as V21) so catalog.Profiles
	// can run against the V9 schema. We cannot migrate past V9 because V14
	// drops and recreates catalog_bridges without re-seeding, which would
	// change the bridge row set for a reason unrelated to V9's enabled
	// column — the exact thing SC-003 isolates.
	if _, err := db.ExecContext(context.Background(),
		`ALTER TABLE catalog_profiles ADD COLUMN sockets INTEGER NOT NULL DEFAULT 1`); err != nil {
		t.Fatalf("add sockets column: %v", err)
	}

	st := store.NewFromDB(db)

	afterNodes, afterStorages, afterBridges, afterISOs, afterProfiles := captureAtLatest(t, st)

	// The guarantee: identical row sets before (V7, direct SQL) and after
	// (V9, catalog functions with AND enabled = 1).
	assertNodesMatch(t, beforeNodes, afterNodes)
	assertStoragesMatch(t, beforeStorages, afterStorages)
	assertBridgesMatch(t, beforeBridges, afterBridges)
	assertISOsMatch(t, beforeISOs, afterISOs)
	assertProfilesMatch(t, beforeProfiles, afterProfiles)
}

func assertNodesMatch(t *testing.T, before []rawNode, after []catalog.Node) {
	t.Helper()

	assertSameLength(t, "nodes", len(before), len(after))

	for i, want := range before {
		if after[i].Name != want.Name {
			t.Fatalf("node[%d]: before=%q after=%q", i, want.Name, after[i].Name)
		}
	}
}

func assertStoragesMatch(t *testing.T, before []rawStorage, after []catalog.Storage) {
	t.Helper()

	assertSameLength(t, "storages", len(before), len(after))

	for i, want := range before {
		if after[i].Name != want.Name || after[i].Node != want.Node {
			t.Fatalf("storage[%d]: before=%+v after=%+v", i, want, after[i])
		}
	}
}

func assertBridgesMatch(t *testing.T, before []rawBridge, after []catalog.Bridge) {
	t.Helper()

	assertSameLength(t, "bridges", len(before), len(after))

	for i, want := range before {
		if after[i].Name != want.Name || after[i].Node != want.Node {
			t.Fatalf("bridge[%d]: before=%+v after=%+v", i, want, after[i])
		}
	}
}

func assertISOsMatch(t *testing.T, before []rawISO, after []catalog.ISO) {
	t.Helper()

	assertSameLength(t, "isos", len(before), len(after))

	for i, want := range before {
		if after[i].Storage != want.Storage || after[i].File != want.File {
			t.Fatalf("iso[%d]: before=%+v after=%+v", i, want, after[i])
		}
	}
}

func assertProfilesMatch(t *testing.T, before []rawProfile, after []catalog.Profile) {
	t.Helper()

	assertSameLength(t, "profiles", len(before), len(after))

	for i, want := range before {
		got := after[i]
		if got.ID != want.ID || got.Label != want.Label || got.CPUCores != want.CPUCores || got.MemoryMB != want.MemoryMB || got.DiskGB != want.DiskGB || got.Bus != want.Bus {
			t.Fatalf("profile[%d]: before=%+v after=%+v", i, want, got)
		}
	}
}

// assertV9EnabledColumnOnAllTables proves V9's ALTER TABLE landed — the enabled
// column must exist on all five catalog tables.
func assertV9EnabledColumnOnAllTables(t *testing.T, db *sql.DB) {
	t.Helper()

	for _, table := range []string{"catalog_nodes", "catalog_storages", "catalog_bridges", "catalog_isos", "catalog_profiles"} {
		if !columnExists(t, db, table, "enabled") {
			t.Fatalf("table %s has no enabled column after migration — V9 not applied", table)
		}
	}
}

// assertPvmssTagSeeded proves catalog_tags was created and seeded with pvmss.
func assertPvmssTagSeeded(t *testing.T, db *sql.DB) {
	t.Helper()

	var tagCount int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM catalog_tags WHERE name = 'pvmss'`).Scan(&tagCount); err != nil {
		t.Fatalf("query catalog_tags: %v", err)
	}

	if tagCount != 1 {
		t.Fatalf("expected 1 pvmss tag row, got %d", tagCount)
	}
}

// assertSameLength fails the test if before and after lengths differ.
func assertSameLength(t *testing.T, label string, before, after int) {
	t.Helper()

	if before != after {
		t.Fatalf("%s count changed: before=%d after=%d", label, before, after)
	}
}
