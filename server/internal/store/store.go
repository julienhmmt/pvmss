package store

import (
	"context"
	"database/sql"
)

// Store is a SQLite-backed persistence handle.
type Store struct {
	db *sql.DB
}

// NewFromDB wraps an already-open *sql.DB in a Store. Used by tests that need
// to control migration application themselves (e.g. the T11 compatibility
// test that captures outputs at V7 then migrates forward to V9). Production
// code should use Open, which runs all migrations.
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
