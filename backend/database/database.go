package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by update/delete operations when the target record
// does not exist in the database.
var ErrNotFound = errors.New("record not found")

// defaultMaxOpenConns is 1 to keep WAL pragma settings on the same connection
// and to serialise all writes (SQLite supports only one writer at a time).
const defaultMaxOpenConns = 1

// DB is the interface for all PVMSS settings persistence operations.
// Every mutating method includes a changedBy parameter (the authenticated
// admin username) that is recorded in the audit_log in the same transaction.
//
//nolint:interfacebloat
type DB interface {
	// Bootstrap
	IsBootstrapComplete() (bool, error)
	CompleteBootstrap(version string) error

	// Resource limits
	GetVMLimits() (*VMLimits, error)
	SetVMLimits(limits *VMLimits, changedBy string) error

	// Per-node limits
	GetNodeLimits() (map[string]NodeLimit, error)
	SetNodeLimit(limit NodeLimit, changedBy string) error
	DeleteNodeLimit(node string, changedBy string) error

	// List-based settings
	GetEnabledNodes() ([]string, error)
	SetEnabledNodes(nodes []string, changedBy string) error
	GetEnabledStorages() ([]string, error)
	SetEnabledStorages(storages []string, changedBy string) error
	GetEnabledISOs() ([]string, error)
	SetEnabledISOs(isos []string, changedBy string) error
	GetEnabledVMBRs() ([]string, error)
	SetEnabledVMBRs(vmbrs []string, changedBy string) error
	GetTags() ([]string, error)
	SetTags(tags []string, changedBy string) error

	// Cloud-init templates
	ListCloudInitTemplates() ([]CloudInitTemplate, error)
	GetCloudInitTemplate(id string) (*CloudInitTemplate, error)
	CreateCloudInitTemplate(t *CloudInitTemplate, changedBy string) error
	UpdateCloudInitTemplate(t *CloudInitTemplate, changedBy string) error
	DeleteCloudInitTemplate(id string, changedBy string) error

	// VM profiles
	ListVMProfiles() ([]VMProfile, error)
	GetVMProfile(id string) (*VMProfile, error)
	CreateVMProfile(p *VMProfile, changedBy string) error
	UpdateVMProfile(p *VMProfile, changedBy string) error
	DeleteVMProfile(id string, changedBy string) error

	// SFTP config
	GetSFTPConfig() (*SFTPConfig, error)
	SetSFTPConfig(cfg *SFTPConfig, changedBy string) error

	// Convenience: assemble full AppSettings from all tables (used to warm cache)
	LoadAppSettings() (*AppSettings, error)

	// Audit
	ListAuditLog(tableFilter string, limit int, offset int) ([]AuditEntry, error)

	// Backup creates a consistent point-in-time snapshot at destPath using VACUUM INTO.
	Backup(destPath string) error

	// RestoreFrom atomically replaces all data in the current database with
	// the contents of the SQLite database at srcPath.
	// The source is validated before any changes are made.
	// changedBy is recorded in the audit log.
	RestoreFrom(srcPath string, changedBy string) error

	Close() error
}

// sqliteDB is the concrete DB implementation backed by modernc.org/sqlite.
type sqliteDB struct {
	db        *sql.DB
	restoreMu sync.Mutex // protects RestoreFrom operations
}

// Open opens (or creates) a file-backed SQLite database at path.
// WAL pragmas are applied and all pending migrations are run before returning.
// The caller is responsible for calling Close when done.
func Open(path string) (DB, error) {
	if path != ":memory:" {
		if path == "" {
			return nil, fmt.Errorf("database path cannot be empty")
		}
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return nil, fmt.Errorf("create db directory %q: %w", dir, err)
			}
		}
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite at %q: %w", path, err)
	}
	raw.SetMaxOpenConns(defaultMaxOpenConns)
	if err := applyPragmas(raw); err != nil {
		_ = raw.Close()
		return nil, err
	}
	store := &sqliteDB{db: raw}
	if err := RunMigrations(store.db); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return store, nil
}

// OpenMemory opens a transient in-memory SQLite database.
// Primarily used in tests; the database is lost when closed.
func OpenMemory() (DB, error) {
	return Open(":memory:")
}

// Close closes the underlying database connection pool.
func (s *sqliteDB) Close() error {
	return s.db.Close()
}

// Backup creates a consistent point-in-time snapshot of the database at
// destPath using SQLite's VACUUM INTO command. Safe to call during live writes.
func (s *sqliteDB) Backup(destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		if err := os.Remove(destPath); err != nil {
			return fmt.Errorf("remove existing backup %q: %w", destPath, err)
		}
	}
	if _, err := s.db.Exec(`VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("backup to %q: %w", destPath, err)
	}
	return nil
}

// applyPragmas sets the production PRAGMA configuration on db.
// Must be called immediately after Open before any other queries.
func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`PRAGMA foreign_keys=OFF`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA cache_size=-64000`,
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}
