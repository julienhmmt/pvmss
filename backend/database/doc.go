// Package database implements the SQLite persistence layer for PVMSS settings.
//
// It provides typed CRUD operations for all user-editable configuration:
// resource limits, enabled nodes/storages/ISOs/bridges, tags, cloud-init
// templates, VM profiles, and SFTP configuration.
//
// Every write is performed inside a transaction that also appends an entry to
// the audit_log table, guaranteeing a consistent audit trail.
//
// # Architecture
//
// The DB interface is the single entry point for callers. Open or OpenMemory
// return a ready-to-use implementation backed by modernc.org/sqlite (pure Go,
// zero CGO).
//
// Schema migrations are applied automatically on Open. The schema_migrations
// table tracks applied versions; migrations are forward-only and idempotent.
//
// # Connection pool
//
// A single write connection (MaxOpenConns=1) is used so that WAL-mode pragma
// settings are never lost across connections. Concurrent readers are served by
// the same connection thanks to WAL mode, which allows reads during active
// writes.
//
// # In-memory cache strategy
//
// This package performs raw DB I/O only. The in-memory settings cache is
// maintained by the StateManager (backend/state), which calls LoadAppSettings
// to warm the cache on startup and after every write.
//
// # Usage
//
//	db, err := database.Open("/app/pvmss.db")
//	if err != nil { ... }
//	defer db.Close()
//
//	limits, err := db.GetVMLimits()
package database
