package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExportDatabase produces a clean, internally consistent snapshot of the live
// SQLite database and streams it to w. It uses VACUUM INTO — a single SQL
// statement that produces a compacted snapshot of a WAL-mode database without
// blocking concurrent readers or writers on the live connection (T14 FR-007,
// constitution VIII: the database's own native mechanism, not hand-rolled
// page-by-page copy code).
//
// The snapshot is written to a temp file in the same directory as the live
// database (so the rename-in-place guarantee holds on the same filesystem),
// then streamed to w, then removed. If w fails mid-stream, the temp file is
// still removed — the snapshot is ephemeral by construction.
func (s *Store) ExportDatabase(ctx context.Context, w io.Writer) error {
	livePath := s.dbPath()
	if livePath == "" {
		return errors.New("export: database path is unknown")
	}

	dir := filepath.Dir(livePath)

	tmpFile, err := os.CreateTemp(dir, "pvmss-export-*.db")
	if err != nil {
		return fmt.Errorf("create export temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	// Close immediately — VACUUM INTO needs the path, not an open handle, and
	// we want to stream it ourselves below.
	_ = tmpFile.Close()

	// Clean up the temp file on any exit path. On success it is removed after
	// streaming; on failure it is removed before returning.
	defer func() { _ = os.Remove(tmpPath) }()

	// VACUUM INTO writes a clean snapshot to the target path. The path is
	// passed as a parameter so the driver handles quoting/escaping.
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tmpPath); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}

	//nolint:gosec // tmpPath is a CreateTemp result, not user input
	file, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open exported snapshot: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("stream exported snapshot: %w", err)
	}

	return nil
}

// dbPath returns the on-disk path of the live database, or empty if it cannot
// be determined. Resolved via PRAGMA database_list — SQLite knows its own
// file path. For an in-memory or non-file DSN, "main" carries an empty file
// and export returns an error before reaching the VACUUM INTO call.
func (s *Store) dbPath() string {
	rows, err := s.db.QueryContext(context.Background(), `PRAGMA database_list`)
	if err != nil {
		return ""
	}
	defer func() { _ = rows.Close() }()

	var path string

	for rows.Next() {
		var (
			seq  int
			name string
			file string
		)
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return ""
		}

		if name == "main" && file != "" {
			path = file
		}
	}

	if err := rows.Err(); err != nil {
		return ""
	}

	return path
}
