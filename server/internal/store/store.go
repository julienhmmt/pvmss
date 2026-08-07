package store

import (
	"context"
	"database/sql"
)

// Store is a SQLite-backed persistence handle.
type Store struct {
	db *sql.DB
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
