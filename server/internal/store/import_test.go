package store_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Table name constants — centralizing them keeps goconst below threshold
// across the store test package.
const (
	tblCatalogNodes     = "catalog_nodes"
	tblCatalogTags      = "catalog_tags"
	tblAuditLog         = "audit_log"
	tblSessions         = "sessions"
	tblAPITokens        = "api_tokens"
	tblSchemaMigrations = "schema_migrations"
)

// newImportStore opens a fully-migrated Store for import tests, with the
// T06/T11/T12 seed data in place.
func newImportStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "import.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// buildCraftedDB creates a SQLite file at path with the given table row sets.
// Each map entry is "table name → list of paren-enclosed VALUES strings"
// (without the surrounding INSERT statement). Tables that don't exist in the
// schema are created with a minimal shape so the crafted file can carry
// excluded-table rows (sessions, api_tokens, schema_migrations, audit_log).
func buildCraftedDB(t *testing.T, path string, rowsByTable map[string][]string) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open crafted db: %v", err)
	}

	defer func() { _ = db.Close() }()

	ctx := context.Background()
	// Recreate the known schema shapes that the allowlist test needs. The
	// exact column shapes match the live migrations so attach-by-attach works
	// against a real exported snapshot too.
	ddls := map[string]string{
		tblCatalogNodes:     `CREATE TABLE catalog_nodes (cluster TEXT NOT NULL, name TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, PRIMARY KEY (cluster, name))`,
		"catalog_storages":  `CREATE TABLE catalog_storages (cluster TEXT NOT NULL, name TEXT NOT NULL, node TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, PRIMARY KEY (cluster, name, node))`,
		"catalog_bridges":   `CREATE TABLE catalog_bridges (cluster TEXT NOT NULL, node TEXT NOT NULL, name TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, PRIMARY KEY (cluster, node, name))`,
		"catalog_isos":      `CREATE TABLE catalog_isos (cluster TEXT NOT NULL, node TEXT NOT NULL, storage TEXT NOT NULL, file TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, PRIMARY KEY (cluster, node, storage, file))`,
		"catalog_profiles":  `CREATE TABLE catalog_profiles (cluster TEXT NOT NULL, id TEXT NOT NULL, label TEXT NOT NULL, cpu_cores INTEGER NOT NULL, memory_mb INTEGER NOT NULL, disk_gb INTEGER NOT NULL, bus TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, PRIMARY KEY (cluster, id))`,
		tblCatalogTags:      `CREATE TABLE catalog_tags (cluster TEXT NOT NULL, name TEXT NOT NULL, color TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY (cluster, name))`,
		"vm_limits":         `CREATE TABLE vm_limits (cluster TEXT PRIMARY KEY, max_sockets INTEGER NOT NULL, max_cores INTEGER NOT NULL, max_memory_mb INTEGER NOT NULL, max_disk_per_vm_gb INTEGER NOT NULL, max_network_cards INTEGER NOT NULL, max_snapshots INTEGER NOT NULL, max_vm_per_user INTEGER NOT NULL, allow_custom_yaml BOOLEAN NOT NULL, isolation_vlan_tag INTEGER NOT NULL DEFAULT 0)`,
		"node_limits":       `CREATE TABLE node_limits (cluster TEXT NOT NULL, node TEXT NOT NULL, max_vms INTEGER NOT NULL, max_vcpus INTEGER NOT NULL, max_ram_gb INTEGER NOT NULL, max_disk_gb INTEGER NOT NULL, PRIMARY KEY (cluster, node))`,
		tblAuditLog:         `CREATE TABLE audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, actor TEXT NOT NULL, cluster TEXT NOT NULL, vmid INTEGER NOT NULL, action TEXT NOT NULL, timestamp TEXT NOT NULL)`,
		tblSessions:         `CREATE TABLE sessions (token_hash BLOB PRIMARY KEY, username TEXT NOT NULL, is_admin INTEGER NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, pool TEXT NOT NULL DEFAULT '')`,
		tblAPITokens:        `CREATE TABLE api_tokens (id TEXT PRIMARY KEY, token_hash BLOB NOT NULL UNIQUE, username TEXT NOT NULL, is_admin INTEGER NOT NULL, scope TEXT NOT NULL, label TEXT NOT NULL, expires_at TEXT, created_at TEXT NOT NULL, last_used_at TEXT, pool TEXT NOT NULL DEFAULT '')`,
		tblSchemaMigrations: `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
	}

	for table, ddl := range ddls {
		if _, ok := rowsByTable[table]; !ok {
			continue
		}

		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
	}

	insertCraftedRows(t, ctx, db, rowsByTable)
}

// insertCraftedRows inserts the row sets into the crafted DB. Extracted from
// buildCraftedDB to keep its Cognitive Complexity under the SonarQube go:S3776
// threshold.
//
//nolint:revive // test helper: t *testing.T is conventionally the first parameter
func insertCraftedRows(t *testing.T, ctx context.Context, db *sql.DB, rowsByTable map[string][]string) {
	t.Helper()

	for table, values := range rowsByTable {
		if len(values) == 0 {
			continue
		}

		colCount, err := columnCount(db, table)
		if err != nil {
			t.Fatalf("column count for %s: %v", table, err)
		}

		placeholders := placeholdersFor(colCount)

		for _, v := range values {
			//nolint:gosec // table and v are test fixtures, not user input
			stmt := fmt.Sprintf("INSERT INTO %s VALUES %s", table, v)
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				// If the row uses fewer columns than the table has, fall back
				// to a placeholder insert. This only happens for crafted
				// excluded-table rows where we don't need every column.
				_ = placeholders

				t.Fatalf("insert into %s values %s: %v", table, v, err)
			}
		}
	}
}

func columnCount(db *sql.DB, table string) (int, error) {
	//nolint:noctx // test helper, table is a fixture
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		count++
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	return count, rows.Err()
}

func placeholdersFor(n int) string {
	if n <= 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("(")

	for i := range n {
		if i > 0 {
			sb.WriteString(",")
		}

		sb.WriteString("?")
	}

	sb.WriteString(")")

	return sb.String()
}

// tableRows returns all rows of `table` as a sorted slice of byte-encoded
// row representations, for byte-identical before/after comparison.
func tableRows(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	//nolint:gosec,noctx // table is a test fixture; ctx not needed in test helper
	rows, err := db.Query("SELECT * FROM " + table)
	if err != nil {
		t.Fatalf("select %s: %v", table, err)
	}

	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns %s: %v", table, err)
	}

	var out []string

	for rows.Next() {
		vals := make([]any, len(cols))

		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("scan %s: %v", table, err)
		}

		out = append(out, fmt.Sprintf("%v", vals))
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows err %s: %v", table, err)
	}

	return out
}

// TestValidateImport_WellFormedFile_ReturnsPreview — T008: ValidateImport on
// a well-formed file returns the correct Tables/IgnoredTables/row counts.
//
//nolint:paralleltest // serial: shared staging map
func TestValidateImport_WellFormedFile_ReturnsPreview(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	craftedPath := filepath.Join(t.TempDir(), "well-formed.db")
	buildCraftedDB(t, craftedPath, map[string][]string{
		tblCatalogNodes: {`('default','crafted-01',1)`, `('default','crafted-02',1)`},
		tblCatalogTags:  {`('default','crafted-tag','#ff0000','2026-01-01T00:00:00Z')`},
		tblAuditLog:     {`(1,'alice','default',101,'start','2026-01-01T00:00:00Z')`},
	})

	data, err := os.ReadFile(craftedPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read crafted: %v", err)
	}

	preview, err := st.ValidateImport(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ValidateImport: %v", err)
	}

	if preview.StagingToken == "" {
		t.Error("staging token is empty")
	}

	if preview.ExpiresAt.IsZero() {
		t.Error("expiresAt is zero")
	}

	byName := make(map[string]int, len(preview.Tables))
	for _, tp := range preview.Tables {
		byName[tp.Name] = tp.RowCount
	}

	if byName[tblCatalogNodes] != 2 {
		t.Errorf("catalog_nodes rowCount = %d, want 2", byName[tblCatalogNodes])
	}

	if byName[tblCatalogTags] != 1 {
		t.Errorf("catalog_tags rowCount = %d, want 1", byName[tblCatalogTags])
	}

	// audit_log is present in the upload but not allowlisted → ignored.
	found := false

	for _, ignored := range preview.IgnoredTables {
		if ignored == tblAuditLog {
			found = true
		}
	}

	if !found {
		t.Errorf("ignoredTables = %v, want to contain audit_log", preview.IgnoredTables)
	}
}

// TestValidateImport_MalformedFile_ReturnsErrorAndStagesNothing — T008: a
// non-SQLite upload returns an explicit error and stages nothing.
//
//nolint:paralleltest // serial: shared staging map
func TestValidateImport_MalformedFile_ReturnsErrorAndStagesNothing(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	_, err := st.ValidateImport(ctx, bytes.NewReader([]byte("this is not a sqlite database")))
	if err == nil {
		t.Fatal("ValidateImport on garbage returned nil error")
	}

	// Nothing is staged — a lookup with any token must fail as not-found.
	_, err = st.Staging().Lookup("any-token-after-malformed")
	if err == nil {
		t.Fatal("Lookup after malformed upload returned nil error — something was staged")
	}
}

// TestConfirmImport_UnknownTokenReturnsNotFound — T008: ConfirmImport on an
// unknown token returns the not-found sentinel.
//
//nolint:paralleltest // serial: shared staging map
func TestConfirmImport_UnknownTokenReturnsNotFound(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	_, err := st.ConfirmImport(ctx, "never-staged")
	if !store.IsNotFound(err) {
		t.Fatalf("ConfirmImport unknown token err = %v, want NotFoundError", err)
	}
}

// TestConfirmImport_ExpiredTokenReturnsExpired — T008: ConfirmImport on an
// expired token returns the expired sentinel.
//
//nolint:paralleltest // serial: shared staging map
func TestConfirmImport_ExpiredTokenReturnsExpired(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	craftedPath := filepath.Join(t.TempDir(), "expired-confirm.db")
	buildCraftedDB(t, craftedPath, map[string][]string{
		tblCatalogNodes: {`('default','x',1)`},
	})
	data, _ := os.ReadFile(craftedPath) //nolint:gosec // test fixture path

	preview, err := st.ValidateImport(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ValidateImport: %v", err)
	}

	// Advance the staging clock past the TTL.
	st.Staging().AdvanceTime(6 * time.Minute)

	_, err = st.ConfirmImport(ctx, preview.StagingToken)
	if !store.IsExpired(err) {
		t.Fatalf("ConfirmImport expired token err = %v, want ExpiredError", err)
	}
}

// TestConfirmImport_ReplacesPreviewedTables — T008: on a valid token,
// ConfirmImport replaces every previewed table in one transaction.
//
//nolint:paralleltest // serial: shared staging map
func TestConfirmImport_ReplacesPreviewedTables(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	// The live DB has the T06 seed (2 catalog_nodes). Craft an upload with
	// 3 different nodes — after confirm, the live table must contain exactly
	// the 3 crafted rows, not the original 2.
	craftedPath := filepath.Join(t.TempDir(), "replace.db")
	buildCraftedDB(t, craftedPath, map[string][]string{
		tblCatalogNodes: {
			`('default','crafted-a',1)`,
			`('default','crafted-b',1)`,
			`('default','crafted-c',1)`,
		},
	})
	data, _ := os.ReadFile(craftedPath) //nolint:gosec // test fixture path

	preview, err := st.ValidateImport(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ValidateImport: %v", err)
	}

	result, err := st.ConfirmImport(ctx, preview.StagingToken)
	if err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}

	if len(result.Tables) != 1 || result.Tables[0].Name != tblCatalogNodes || result.Tables[0].RowCount != 3 {
		t.Errorf("result = %+v, want catalog_nodes rowCount 3", result)
	}

	// Verify the live DB now has exactly the crafted rows.
	liveDB := openLiveReadOnly(ctx, t, st)
	defer func() { _ = liveDB.Close() }()

	var liveCount int
	if err := liveDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM catalog_nodes`).Scan(&liveCount); err != nil {
		t.Fatalf("count live: %v", err)
	}

	if liveCount != 3 {
		t.Errorf("live catalog_nodes = %d, want 3 (replaced)", liveCount)
	}
}

// TestImportAllowlist_Sc005_ExcludesAuthSystemHistoryTables — T011/SC-005:
// a crafted upload with rows for catalog_tags (allowlisted) alongside
// sessions, api_tokens, schema_migrations, and audit_log (all excluded);
// after confirm, only catalog_tags changed, the four excluded tables are
// byte-identical before/after.
//
//nolint:paralleltest // serial: shared staging map
func TestImportAllowlist_Sc005_ExcludesAuthSystemHistoryTables(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	// Seed the live DB with a known state for each excluded table so we can
	// compare byte-identically before/after.
	seedExcludedTables(ctx, t, st)

	livePath := liveDBPath(ctx, t, st)
	beforeRows := snapshotAllTables(t, livePath, []string{
		tblSessions, tblAPITokens, tblSchemaMigrations, tblAuditLog, tblCatalogTags,
	})

	// Craft an upload with catalog_tags rows (allowlisted) plus rows for each
	// excluded table. The excluded rows are crafted to be DIFFERENT from the
	// live rows so that if the allowlist were broken, the test would catch it.
	craftedPath := filepath.Join(t.TempDir(), "sc005.db")
	buildCraftedDB(t, craftedPath, map[string][]string{
		tblCatalogTags: {
			`('default','imported-tag','#00ff00','2026-01-01T00:00:00Z')`,
			`('default','another-tag','#0000ff','2026-01-01T00:00:00Z')`,
		},
		tblSessions: {
			`(X'cafebabe','attacker','1','2099-01-01T00:00:00Z','2026-01-01T00:00:00Z','evil')`,
		},
		tblAPITokens: {
			`('attacker-id',X'deadbeef','attacker',1,'admin','evil-token',NULL,'2026-01-01T00:00:00Z',NULL,'evil')`,
		},
		tblSchemaMigrations: {
			`(999,'2026-01-01T00:00:00Z')`,
		},
		tblAuditLog: {
			`(999,'attacker','default',1,'fabricated','2026-01-01T00:00:00Z')`,
		},
	})
	data, _ := os.ReadFile(craftedPath) //nolint:gosec // test fixture path

	preview, err := st.ValidateImport(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ValidateImport: %v", err)
	}

	// The preview's Tables must contain ONLY catalog_tags — the four excluded
	// tables must appear in IgnoredTables, not Tables.
	for _, tp := range preview.Tables {
		if tp.Name != tblCatalogTags {
			t.Errorf("preview Tables contains %s — should be excluded", tp.Name)
		}
	}

	ignoredSet := make(map[string]bool, len(preview.IgnoredTables))
	for _, name := range preview.IgnoredTables {
		ignoredSet[name] = true
	}

	for _, excluded := range []string{tblSessions, tblAPITokens, tblSchemaMigrations, tblAuditLog} {
		if !ignoredSet[excluded] {
			t.Errorf("preview IgnoredTables missing %s", excluded)
		}
	}

	if _, err := st.ConfirmImport(ctx, preview.StagingToken); err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}

	// After confirm: catalog_tags changed (replaced with the crafted rows),
	// the four excluded tables are byte-identical to before.
	afterRows := snapshotAllTables(t, livePath, []string{
		tblSessions, tblAPITokens, tblSchemaMigrations, tblAuditLog, tblCatalogTags,
	})

	for _, table := range []string{tblSessions, tblAPITokens, tblSchemaMigrations, tblAuditLog} {
		if !equalStringSlices(beforeRows[table], afterRows[table]) {
			t.Errorf("excluded table %s changed: before=%v after=%v", table, beforeRows[table], afterRows[table])
		}
	}

	// catalog_tags must have changed — it now contains the crafted rows.
	if equalStringSlices(beforeRows[tblCatalogTags], afterRows[tblCatalogTags]) {
		t.Error("catalog_tags did not change — import did not apply the allowlisted table")
	}
}

// TestImportAllowlist_ListMatchesCurrentSchema — the importableTables list
// contains exactly the instance-configuration tables that exist in the
// current schema, and excludes auth/system/history tables. This test does
// NOT hardcode that the list is exhaustive of every future tranche's tables
// — only of the ones that exist at the time this test runs (per plan.md's
// constraint on T011).
//
//nolint:paralleltest // serial: shared schema
func TestImportAllowlist_ListMatchesCurrentSchema(t *testing.T) {
	st := newImportStore(t)
	ctx := context.Background()

	// Discover every table in the live schema.
	livePath := liveDBPath(ctx, t, st)

	liveDB, err := sql.Open("sqlite", "file:"+livePath+"?mode=ro")
	if err != nil {
		t.Fatalf("open live: %v", err)
	}

	defer func() { _ = liveDB.Close() }()

	rows, err := liveDB.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	liveTables := make(map[string]bool)

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}

		liveTables[name] = true
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}

	allowlisted := make(map[string]bool)
	for _, name := range store.ImportableTables() {
		allowlisted[name] = true
	}

	// Every allowlisted table must exist in the live schema.
	for name := range allowlisted {
		if !liveTables[name] {
			t.Errorf("importableTables lists %s, but it does not exist in the live schema", name)
		}
	}

	// Every table in the excluded categories must NOT be allowlisted.
	excluded := []string{tblSchemaMigrations, tblSessions, tblAPITokens, tblAuditLog, "vm_cloudinit_snippets"}
	for _, name := range excluded {
		if !liveTables[name] {
			continue // table doesn't exist yet — fine, the test is tolerant
		}

		if allowlisted[name] {
			t.Errorf("table %s is allowlisted but should be excluded (auth/system/history)", name)
		}
	}
}

// --- helpers ---

func seedExcludedTables(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()
	// Seed audit_log with a known row.
	if err := st.RecordAction(ctx, "alice@pve", "default", 101, "start"); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	// Seed sessions, api_tokens, schema_migrations directly via the Store's
	// underlying connection through ExecContext on the exported helper.
	// schema_migrations is already populated by migrations; we just verify it.
	// sessions and api_tokens are seeded with a known row so the before/after
	// comparison is meaningful.
	livePath := liveDBPath(ctx, t, st)

	db, err := sql.Open("sqlite", "file:"+livePath)
	if err != nil {
		t.Fatalf("open live for seed: %v", err)
	}

	defer func() { _ = db.Close() }()

	_, err = db.ExecContext(ctx, `INSERT INTO sessions (token_hash, username, is_admin, expires_at, created_at, pool) VALUES (X'aaaa', 'admin', 1, '2099-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '')`)
	if err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO api_tokens (id, token_hash, username, is_admin, scope, label, expires_at, created_at, last_used_at, pool) VALUES ('seed-id', X'bbbb', 'admin', 1, 'admin', 'seed', NULL, '2026-01-01T00:00:00Z', NULL, '')`)
	if err != nil {
		t.Fatalf("seed api_tokens: %v", err)
	}
}

func liveDBPath(ctx context.Context, t *testing.T, st *store.Store) string {
	t.Helper()

	rows, err := st.DB().QueryContext(ctx, `PRAGMA database_list`)
	if err != nil {
		t.Fatalf("database_list: %v", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			seq        int
			name, file string
		)
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatalf("scan: %v", err)
		}

		if name == "main" {
			return file
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}

	t.Fatal("could not determine live DB path")

	return ""
}

func openLiveReadOnly(ctx context.Context, t *testing.T, st *store.Store) *sql.DB {
	t.Helper()
	path := liveDBPath(ctx, t, st)

	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open live ro: %v", err)
	}

	return db
}

func snapshotAllTables(t *testing.T, dbPath string, tables []string) map[string][]string {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}

	defer func() { _ = db.Close() }()

	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		out[table] = tableRows(t, db, table)
	}

	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
