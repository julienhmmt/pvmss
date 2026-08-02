package store

import "database/sql"

// Store is a SQLite-backed persistence handle.
type Store struct {
	db *sql.DB
}

// Ping verifies the database connection is alive.
func (s *Store) Ping() error {
	return s.db.Ping()
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
