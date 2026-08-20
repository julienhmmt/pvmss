package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// importableTables is the literal allowlist of PVMSS instance-configuration
// tables that an import may replace (T14 data-model.md). Auth/system
// bookkeeping (schema_migrations, sessions, api_tokens), historical/per-VM
// runtime data (audit_log, vm_cloudinit_snippets), and any table not on this
// list are excluded — see data-model.md's three-way category test.
//
// T12 and T13 each append their own table name(s) here once their migrations
// land — not invented by this tranche (spec Assumptions). The list is
// intentionally exported via ImportableTables() so tests can verify it
// against the live schema without reaching into package-private state.
var importableTables = []string{
	"catalog_nodes",
	"catalog_storages",
	"catalog_bridges",
	"catalog_isos",
	"catalog_profiles",
	"catalog_tags",
	"vm_limits",
	"node_limits",
}

// ImportableTables returns a copy of the import allowlist. Tests use this to
// verify the list matches the live schema without hardcoding an exhaustive
// expectation of every future tranche's tables (plan.md constraint on T011).
func ImportableTables() []string {
	out := make([]string, len(importableTables))
	copy(out, importableTables)

	return out
}

// TablePreview is one row of the import preview: an allowlisted table present
// in the upload and the number of rows it carries.
type TablePreview struct {
	Name     string `json:"name"`
	RowCount int    `json:"rowCount"`
}

// ImportPreview is the result of ValidateImport — what the admin sees before
// confirming. Tables lists allowlisted-and-present tables with row counts;
// IgnoredTables lists tables present in the upload but not allowlisted,
// shown for transparency, never applied.
type ImportPreview struct {
	StagingToken  string
	ExpiresAt     time.Time
	Tables        []TablePreview
	IgnoredTables []string
}

// ImportResult is the outcome of a successful ConfirmImport — the tables that
// were replaced and their row counts.
type ImportResult struct {
	Tables []TablePreview
}

// importableSet is the lookup form of importableTables.
var importableSet = func() map[string]bool {
	m := make(map[string]bool, len(importableTables))
	for _, t := range importableTables {
		m[t] = true
	}

	return m
}()

// ValidateImport writes the upload to a temp file, opens it read-only as a
// second SQLite connection, lists its tables via sqlite_master, intersects
// with importableTables, counts rows per matching table, and stages the
// result in the in-memory ImportStaging (T14 FR-008/FR-009). Nothing in the
// live database is touched.
//
// A non-SQLite upload returns ErrInvalidDatabase and stages nothing.
func (s *Store) ValidateImport(ctx context.Context, upload io.Reader) (ImportPreview, error) {
	// Write the upload to a temp file in the live DB's directory so the
	// staging entry's temp path shares a filesystem with the live database.
	livePath := s.dbPath()
	if livePath == "" {
		return ImportPreview{}, errors.New("import: database path is unknown")
	}

	dir := filepath.Dir(livePath)

	tmpFile, err := os.CreateTemp(dir, "pvmss-import-*.db")
	if err != nil {
		return ImportPreview{}, fmt.Errorf("create import temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	// Clean up the temp file on any failure path. On success, the staging
	// entry owns it and ConfirmImport removes it after applying (or the TTL
	// purge removes it lazily on the next staging access).
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmpFile, upload); err != nil {
		return ImportPreview{}, fmt.Errorf("write upload: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return ImportPreview{}, fmt.Errorf("close upload temp file: %w", err)
	}

	// Open the uploaded file read-only as a second, independent SQLite
	// connection — never merged into the live connection pool.
	uploadDB, err := sql.Open("sqlite", "file:"+tmpPath+"?mode=ro")
	if err != nil {
		return ImportPreview{}, fmt.Errorf("open upload: %w", err)
	}
	defer func() { _ = uploadDB.Close() }()

	// Probe: a non-SQLite file fails this lightweight integrity check.
	// integrity_check returns "ok" for a valid database; any error or
	// non-"ok" result means the upload is not a usable SQLite file.
	var integrityResult string
	if err := uploadDB.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrityResult); err != nil {
		return ImportPreview{}, ErrInvalidDatabase
	}

	if integrityResult != "ok" {
		return ImportPreview{}, ErrInvalidDatabase
	}

	// List every user table in the upload and split into allowlisted/ignored.
	tables, ignoredTables, err := classifyUploadTables(ctx, uploadDB)
	if err != nil {
		return ImportPreview{}, err
	}

	// Stage the preview. The staging entry owns the temp file from here.
	token := s.staging.Stage(tmpPath, ImportPreview{
		Tables:        tables,
		IgnoredTables: ignoredTables,
	})
	cleanup = false

	entry, err := s.staging.Lookup(token)
	if err != nil {
		// Should be impossible right after Stage, but guard anyway.
		return ImportPreview{}, err
	}

	return ImportPreview{
		StagingToken:  token,
		ExpiresAt:     entry.expiresAt,
		Tables:        tables,
		IgnoredTables: ignoredTables,
	}, nil
}

// classifyUploadTables lists every user table in the upload via sqlite_master,
// then splits into allowlisted tables (with row counts) and ignored tables.
// Table names come from sqlite_master, not user input — the gosec G201/G202
// nolints on the COUNT(*) query reflect this.
func classifyUploadTables(ctx context.Context, uploadDB *sql.DB) ([]TablePreview, []string, error) {
	rows, err := uploadDB.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, nil, fmt.Errorf("list upload tables: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var presentTables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, nil, fmt.Errorf("scan upload table: %w", err)
		}

		presentTables = append(presentTables, name)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate upload tables: %w", err)
	}

	if err := rows.Close(); err != nil {
		return nil, nil, fmt.Errorf("close upload tables: %w", err)
	}

	var (
		tables        []TablePreview
		ignoredTables []string
	)

	for _, name := range presentTables {
		if !importableSet[name] {
			ignoredTables = append(ignoredTables, name)
			continue
		}

		var rowCount int
		if err := uploadDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+name).Scan(&rowCount); err != nil {
			return nil, nil, fmt.Errorf("count %s: %w", name, err)
		}

		tables = append(tables, TablePreview{Name: name, RowCount: rowCount})
	}

	sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })
	sort.Strings(ignoredTables)

	return tables, ignoredTables, nil
}

// ConfirmImport looks up the staging token (404 if unknown, 410 if expired)
// and, in one SQLite transaction against the live database, replaces every
// table named in the preview — DELETE then reload from the staged file —
// all-or-nothing (T14 FR-010). On success, the temp file and staging entry
// are removed. On failure, the transaction rolls back and the staging entry
// is kept so the admin may retry confirm without re-uploading.
func (s *Store) ConfirmImport(ctx context.Context, token string) (ImportResult, error) {
	entry, err := s.staging.Lookup(token)
	if err != nil {
		return ImportResult{}, err
	}

	// Open the staged file read-only for row copies.
	uploadDB, err := sql.Open("sqlite", "file:"+entry.TempPath+"?mode=ro")
	if err != nil {
		return ImportResult{}, fmt.Errorf("open staged file: %w", err)
	}
	defer func() { _ = uploadDB.Close() }()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, fmt.Errorf("begin import transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var applied []TablePreview

	for _, tp := range entry.Preview.Tables {
		if err := replaceTable(ctx, tx, uploadDB, tp.Name); err != nil {
			return ImportResult{}, fmt.Errorf("replace %s: %w", tp.Name, err)
		}

		applied = append(applied, tp)
	}

	if err := tx.Commit(); err != nil {
		return ImportResult{}, fmt.Errorf("commit import: %w", err)
	}

	committed = true

	// Success: remove the temp file and the staging entry.
	s.staging.Remove(token)
	_ = os.Remove(entry.TempPath)

	return ImportResult{Tables: applied}, nil
}

// replaceTable deletes every row of `table` from the live DB (within tx) and
// re-inserts every row copied from the staged upload DB. Table and column
// names come from sqlite_master introspection, never user input.
func replaceTable(ctx context.Context, tx *sql.Tx, uploadDB *sql.DB, table string) error {
	// Introspect the upload table's columns — the live schema is the same
	// (the upload came from an export of this schema), but reading from the
	// upload avoids assuming the live schema's column order matches.
	cols, err := tableColumns(ctx, uploadDB, table)
	if err != nil {
		return fmt.Errorf("introspect %s: %w", table, err)
	}

	if len(cols) == 0 {
		return fmt.Errorf("table %s has no columns", table)
	}

	//nolint:gosec // table name comes from sqlite_master, validated against importableTables
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
		return fmt.Errorf("delete from %s: %w", table, err)
	}

	return streamRowsIntoLive(ctx, tx, uploadDB, table, cols)
}

// streamRowsIntoLive streams every row of `table` from the upload DB into the
// live DB (within tx), using `cols` as the column list. Extracted from
// replaceTable to keep its Cognitive Complexity under the SonarQube go:S3776
// threshold. Table and column names come from sqlite_master/PRAGMA, not user
// input.
//

func streamRowsIntoLive(ctx context.Context, tx *sql.Tx, uploadDB *sql.DB, table string, cols []string) error {
	selectSQL, insertSQL := tableSelectInsertSQL(table, cols)

	rows, err := uploadDB.QueryContext(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("select %s from upload: %w", table, err)
	}

	defer func() { _ = rows.Close() }()

	// Allocate one scan destination per column. Using *any lets the driver
	// choose the right Go type per column (string, int64, []byte, etc.) and
	// pass it through to the insert unchanged.
	scanArgs := make([]any, len(cols))
	for i := range scanArgs {
		scanArgs[i] = new(any)
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("scan %s: %w", table, err)
		}

		insertArgs, err := dereferenceScanArgs(table, scanArgs)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, insertSQL, insertArgs...); err != nil {
			return fmt.Errorf("insert into %s: %w", table, err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s: %w", table, err)
	}

	return nil
}

// tableSelectInsertSQL builds the SELECT (against the upload DB) and INSERT
// (against the live DB) statements for `table` with columns `cols`. Extracted
// from replaceTable for SonarQube go:S3776.
//

func tableSelectInsertSQL(table string, cols []string) (selectSQL, insertSQL string) {
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "?"
	}

	selectSQL = fmt.Sprintf(`SELECT %s FROM %s`, strings.Join(quoteCols(cols), ", "), table)
	insertSQL = fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		table,
		strings.Join(quoteCols(cols), ", "),
		strings.Join(placeholders, ", "),
	)

	return selectSQL, insertSQL
}

// dereferenceScanArgs converts the []*any scan destinations back into a slice
// of concrete values for the INSERT. Returns an error if a scan destination
// has an unexpected type (should never happen given scanArgs is built by
// streamRowsIntoLive, but guards against future regressions).
func dereferenceScanArgs(table string, scanArgs []any) ([]any, error) {
	insertArgs := make([]any, len(scanArgs))
	for i, v := range scanArgs {
		ptr, ok := v.(*any)
		if !ok {
			return nil, fmt.Errorf("scan %s: unexpected scan destination type %T", table, v)
		}

		insertArgs[i] = *ptr
	}

	return insertArgs, nil
}

// tableColumns returns the column names of `table` from db, in definition
// order. `table` must be a real table in db (validated by the caller via
// sqlite_master).
func tableColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cols []string

	for rows.Next() {
		var (
			cid       int
			name      string
			declType  string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &declType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		cols = append(cols, name)
	}

	return cols, rows.Err()
}

// quoteCols wraps each column name in double quotes for safe SQL emission.
// Column names come from PRAGMA table_info, never user input.
func quoteCols(cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
	}

	return out
}

// ErrInvalidDatabase is returned by ValidateImport when the upload is not a
// well-formed SQLite database (T14 FR-009).
var ErrInvalidDatabase = errors.New("uploaded file is not a valid SQLite database")
