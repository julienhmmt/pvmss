// Package store provides SQLite-backed persistence and schema migrations.
package store

// schemaV1 creates the schema_migrations tracking table.
const schemaV1 = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
)`

const schemaV2 = `CREATE TABLE api_tokens (
	id TEXT PRIMARY KEY,
	token_hash BLOB NOT NULL UNIQUE,
	username TEXT NOT NULL,
	is_admin INTEGER NOT NULL,
	scope TEXT NOT NULL,
	label TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_used_at TEXT
)`

const schemaV3 = `CREATE TABLE sessions (
	token_hash BLOB PRIMARY KEY,
	username TEXT NOT NULL,
	is_admin INTEGER NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL
)`

// schemaV4 drops the NOT NULL constraint on api_tokens.expires_at: creation
// tokens no longer take an expiry input (contracts/auth-tokens.md), so the
// column must accept NULL. SQLite has no ALTER COLUMN, so the table is rebuilt.
const schemaV4 = `
ALTER TABLE api_tokens RENAME TO api_tokens_v2;
CREATE TABLE api_tokens (
	id TEXT PRIMARY KEY,
	token_hash BLOB NOT NULL UNIQUE,
	username TEXT NOT NULL,
	is_admin INTEGER NOT NULL,
	scope TEXT NOT NULL,
	label TEXT NOT NULL,
	expires_at TEXT,
	created_at TEXT NOT NULL,
	last_used_at TEXT
);
INSERT INTO api_tokens (id, token_hash, username, is_admin, scope, label, expires_at, created_at, last_used_at)
	SELECT id, token_hash, username, is_admin, scope, label, expires_at, created_at, last_used_at FROM api_tokens_v2;
DROP TABLE api_tokens_v2;
`

// schemaV5 adds the tenancy anchor (T02 data-model: one pool per user) to
// both identity stores, so T04's scoped reads can resolve ByPool[identity.Pool].
const schemaV5 = `
ALTER TABLE sessions ADD COLUMN pool TEXT NOT NULL DEFAULT '';
ALTER TABLE api_tokens ADD COLUMN pool TEXT NOT NULL DEFAULT '';
`

// schemaV6 adds the audit_log table (T05 FR-009). Every VM write flows through
// Resolve() and is recorded here with the real acting user — closing the
// traceability gap S01's document names as the reason the flaw went undetected.
// Write-only from T05's perspective; a read endpoint belongs to T14.
const schemaV6 = `CREATE TABLE audit_log (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	actor     TEXT NOT NULL,
	cluster   TEXT NOT NULL,
	vmid      INTEGER NOT NULL,
	action    TEXT NOT NULL,
	timestamp TEXT NOT NULL
)`

// Migration is a single schema version and its forward-only DDL.
type Migration struct {
	Version int
	DDL     string
}

// Migrations is the ordered list of schema migrations.
// Versions must be consecutive integers starting at 1.
var Migrations = []Migration{
	{Version: 1, DDL: schemaV1},
	{Version: 2, DDL: schemaV2},
	{Version: 3, DDL: schemaV3},
	{Version: 4, DDL: schemaV4},
	{Version: 5, DDL: schemaV5},
	{Version: 6, DDL: schemaV6},
	{Version: 7, DDL: schemaV7},
	{Version: 8, DDL: schemaV8},
	{Version: 9, DDL: schemaV9},
	{Version: 10, DDL: schemaV10},
}
