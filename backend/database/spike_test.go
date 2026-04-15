// Package database spike: validates that modernc.org/sqlite (pure Go, zero CGO)
// compiles and behaves correctly for the WAL-mode, concurrent-access patterns
// required by the settings migration architecture.
//
// Run with:
//
//	CGO_ENABLED=0 go test -v -race ./database/
//	CGO_ENABLED=0 go test -bench=. -benchmem ./database/
package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openSpikeDB opens an in-memory SQLite database with production PRAGMAs.
// MaxOpenConns=1 ensures the pragma-configured connection is always reused
// (models the production single-write-connection pattern).
func openSpikeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := applyPragmas(db); err != nil {
		t.Fatalf("applyPragmas: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ── Smoke test ────────────────────────────────────────────────────────────────

// TestSpikeSmoke is T002: open DB, create table, insert a row, read it back.
// A failure here means the driver itself does not work under CGO_ENABLED=0.
func TestSpikeSmoke(t *testing.T) {
	db := openSpikeDB(t)

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}

	_, err = db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?)`, "hello", "world")
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	var val string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, "hello").Scan(&val); err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if val != "world" {
		t.Errorf("got %q; want %q", val, "world")
	}
}

// TestSpikePragmaWAL verifies that journal_mode is indeed WAL after applying
// the production pragma set (sanity-check for distroless environments).
func TestSpikePragmaWAL(t *testing.T) {
	db := openSpikeDB(t)

	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	// :memory: databases report "memory" even when WAL is set — this is expected
	// SQLite behaviour. On a file-backed DB the value would be "wal".
	// We accept both here; the important thing is the driver didn't error.
	if mode != "wal" && mode != "memory" {
		t.Errorf("unexpected journal_mode %q; want wal or memory", mode)
	}
}

// TestSpikeTransaction verifies that a transaction either commits or rolls back
// atomically — critical for the "write DB then refresh cache" pattern.
func TestSpikeTransaction(t *testing.T) {
	db := openSpikeDB(t)
	_, err := db.Exec(`CREATE TABLE counters (name TEXT PRIMARY KEY, n INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	_, err = db.Exec(`INSERT INTO counters (name, n) VALUES ('hits', 0)`)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	t.Run("commit", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		if _, err := tx.Exec(`UPDATE counters SET n = n + 1 WHERE name = 'hits'`); err != nil {
			_ = tx.Rollback()
			t.Fatalf("UPDATE: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		var n int
		if err := db.QueryRow(`SELECT n FROM counters WHERE name = 'hits'`).Scan(&n); err != nil {
			t.Fatalf("SELECT: %v", err)
		}
		if n != 1 {
			t.Errorf("after commit got n=%d; want 1", n)
		}
	})

	t.Run("rollback", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		if _, err := tx.Exec(`UPDATE counters SET n = 999 WHERE name = 'hits'`); err != nil {
			_ = tx.Rollback()
			t.Fatalf("UPDATE: %v", err)
		}
		_ = tx.Rollback()

		var n int
		if err := db.QueryRow(`SELECT n FROM counters WHERE name = 'hits'`).Scan(&n); err != nil {
			t.Fatalf("SELECT: %v", err)
		}
		if n != 1 {
			t.Errorf("after rollback got n=%d; want 1 (rollback should have no effect)", n)
		}
	})
}

// ── Concurrency test (T008 / T009) ───────────────────────────────────────────

// openFileDB opens a file-backed SQLite database in a temp directory.
// WAL mode is only meaningful on file-backed databases; :memory: ignores it.
// MaxOpenConns=1 keeps the pragma-configured connection active and models the
// production single-write-connection pattern.
func openFileDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open (file): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := applyPragmas(db); err != nil {
		t.Fatalf("applyPragmas: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestSpikeConcurrentReadsDuringWrite validates the WAL-mode promise:
// concurrent readers must not error while a single writer is active.
//
// Uses a file-backed DB because :memory: ignores WAL and serialises access.
//
// Simulates the production pattern: background Proxmox cache workers
// reading settings while an admin write triggers a cache reload.
func TestSpikeConcurrentReadsDuringWrite(t *testing.T) {
	db := openFileDB(t)

	_, err := db.Exec(`CREATE TABLE settings (k TEXT PRIMARY KEY, v TEXT NOT NULL)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	_, err = db.Exec(`INSERT INTO settings VALUES ('key', 'initial')`)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	// Verify WAL is active on this file-backed DB.
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected journal_mode=wal on file DB, got %q", mode)
	}

	const (
		readers  = 20
		duration = 300 * time.Millisecond
	)

	var (
		wg          sync.WaitGroup
		readErrors  atomic.Int64
		writeErrors atomic.Int64
	)

	// Start concurrent readers.
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				var v string
				if err := db.QueryRow(`SELECT v FROM settings WHERE k = 'key'`).Scan(&v); err != nil {
					readErrors.Add(1)
					return
				}
			}
		}()
	}

	// Single writer (simulates admin save → cache reload cycle).
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(duration)
		i := 0
		for time.Now().Before(deadline) {
			if _, err := db.Exec(`UPDATE settings SET v = ? WHERE k = 'key'`, fmt.Sprintf("update-%d", i)); err != nil {
				writeErrors.Add(1)
				return
			}
			i++
		}
	}()

	wg.Wait()

	if n := readErrors.Load(); n > 0 {
		t.Errorf("concurrent reads produced %d error(s)", n)
	}
	if n := writeErrors.Load(); n > 0 {
		t.Errorf("concurrent writes produced %d error(s)", n)
	}
}

// ── Benchmarks (T006 / T007) ──────────────────────────────────────────────────

// inMemoryCache simulates the StateManager in-memory settings cache:
// a simple struct protected by a RWMutex, representing zero-DB-latency reads.
type inMemoryCache struct {
	mu    sync.RWMutex
	value string
}

func (c *inMemoryCache) get() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// BenchmarkCacheVsSQLite compares 1 000 reads from in-memory cache vs
// 1 000 reads from SQLite to confirm the caching strategy is justified.
//
// Expected: cache <1 µs/op, SQLite ~10–50 µs/op.
func BenchmarkCacheVsSQLite(b *testing.B) {
	// Setup SQLite
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE s (k TEXT PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		b.Fatalf("CREATE TABLE: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO s VALUES ('cfg', 'value')`); err != nil {
		b.Fatalf("INSERT: %v", err)
	}

	// Setup cache
	cache := &inMemoryCache{value: "value"}

	b.Run("memory_cache_read", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_ = cache.get()
		}
	})

	b.Run("sqlite_read", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			var v string
			if err := db.QueryRow(`SELECT v FROM s WHERE k = 'cfg'`).Scan(&v); err != nil {
				b.Fatalf("SELECT: %v", err)
			}
		}
	})
}

// BenchmarkSQLiteWrite measures a single-row UPDATE (the hot path for any
// admin settings change) to confirm write latency is acceptable.
func BenchmarkSQLiteWrite(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE s (k TEXT PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		b.Fatalf("CREATE TABLE: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO s VALUES ('cfg', 'initial')`); err != nil {
		b.Fatalf("INSERT: %v", err)
	}

	b.ResetTimer()
	for i := range b.N {
		if _, err := db.Exec(`UPDATE s SET v = ? WHERE k = 'cfg'`, fmt.Sprintf("v%d", i)); err != nil {
			b.Fatalf("UPDATE: %v", err)
		}
	}
}
