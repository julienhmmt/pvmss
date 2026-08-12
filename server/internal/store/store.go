package store

import (
	"context"
	"database/sql"
)

// Store is a SQLite-backed persistence handle.
type Store struct {
	db            *sql.DB
	staging       *ImportStaging
	encryptionKey []byte
}

// NewFromDB wraps an already-open *sql.DB in a Store. Used by tests that need
// to control migration application themselves (e.g. the T11 compatibility
// test that captures outputs at V7 then migrates forward to V9). Production
// code should use Open, which runs all migrations.
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db, staging: NewImportStaging()}
}

// NewFromDBWithSecret wraps an open database and derives the cluster-token
// encryption key from the supplied session secret.
func NewFromDBWithSecret(db *sql.DB, sessionSecret string) (*Store, error) {
	key, err := deriveEncryptionKey(sessionSecret)
	if err != nil {
		return nil, err
	}

	return &Store{db: db, staging: NewImportStaging(), encryptionKey: key}, nil
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Staging returns the import-preview staging map. Used by ValidateImport and
// ConfirmImport; exposed for tests that need to drive the staging clock or
// inspect its state directly.
func (s *Store) Staging() *ImportStaging {
	if s.staging == nil {
		s.staging = NewImportStaging()
	}

	return s.staging
}

// DB returns the underlying *sql.DB. Exposed for tests that need to introspect
// the live schema (e.g. PRAGMA database_list to find the file path). Not used
// by production callers outside this package.
func (s *Store) DB() *sql.DB {
	return s.db
}
