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

// schemaV11 adds runtime-managed cluster records and records the cluster used
// by browser sessions. The exact version follows T14's latest migration.
const schemaV11 = `
CREATE TABLE clusters (
	name                       TEXT PRIMARY KEY,
	url                        TEXT NOT NULL,
	tls_insecure_skip_verify   INTEGER NOT NULL DEFAULT 0,
	token_id                   TEXT NOT NULL,
	token_secret_ciphertext    BLOB,
	oidc_enabled               INTEGER NOT NULL DEFAULT 0,
	created_at                 TEXT NOT NULL,
	removed_at                 TEXT,
	last_test_status           TEXT,
	last_test_at               TEXT,
	last_test_message          TEXT,
	proxmox_version            TEXT
);
ALTER TABLE sessions ADD COLUMN cluster TEXT NOT NULL DEFAULT '';
`

// schemaV12 adds the admin-curated cloud-init template catalog (T18): a sibling
// to T11's catalog_profiles, full CRUD with no Proxmox-side discovery source.
// The version is provisional (plan.md Constraints) — the exact integer is fixed
// by actual merge order, not spec-writing order.
const schemaV12 = `CREATE TABLE catalog_cloudinit_templates (
	cluster    TEXT NOT NULL,
	id         TEXT NOT NULL,
	label      TEXT NOT NULL,
	content    TEXT NOT NULL,
	enabled    BOOLEAN NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (cluster, id)
)`

// schemaV13 adds admin-authored documentation pages (issue #53): Markdown
// pages with a user/admin audience, an enabled toggle, and a built-in
// is_system flag that protects seeded pages from delete or id/lang change.
// The composite PK (id, lang) lets the same page exist in multiple languages;
// the en row is the fallback when a requested lang is absent.
const schemaV13 = `CREATE TABLE documentation_pages (
	id         TEXT NOT NULL,
	lang       TEXT NOT NULL DEFAULT 'en',
	title      TEXT NOT NULL,
	category   TEXT,
	body_md    TEXT NOT NULL,
	audience   TEXT NOT NULL DEFAULT 'user',
	enabled    BOOLEAN NOT NULL DEFAULT 1,
	is_system  BOOLEAN NOT NULL DEFAULT 0,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT,
	updated_at TEXT,
	PRIMARY KEY (id, lang),
	CHECK (audience IN ('user','admin'))
)`

const schemaV14 = `
DROP TABLE catalog_bridges;
CREATE TABLE catalog_bridges (
	cluster TEXT NOT NULL,
	node    TEXT NOT NULL,
	name    TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, node, name)
);
`

// schemaV15 records the pools PVMSS has provisioned so deletion can be scoped
// to managed pools only. A row is written only after pools.Create succeeds
// end-to-end; pre-existing Proxmox pools are intentionally not adopted because
// the legacy schema stored no reliable PVMSS-origin marker (issue #5).
const schemaV15 = `CREATE TABLE managed_pools (
	cluster    TEXT NOT NULL,
	name       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (cluster, name)
)`

// schemaV16 adds a display_name column to clusters so the UI can show the real
// Proxmox cluster name (from /cluster/status) alongside the immutable internal
// key. Existing rows get NULL — callers fall back to the logical name.
const schemaV16 = `ALTER TABLE clusters ADD COLUMN display_name TEXT`

// schemaV17 rebuilds catalog_isos with node in the primary key. The previous
// schema keyed on (cluster, storage, file), which collapsed duplicate ISO files
// discovered on the same storage name across multiple nodes — the toggle could
// not be attributed to a single node and the UI showed duplicate Svelte keys.
// Node is now part of the identity, mirroring catalog_bridges' (cluster, node,
// name) key. Existing rows are dropped (they are repopulated by discovery +
// admin approval; the recovery tool writes node="" for legacy entries).
const schemaV17 = `
DROP TABLE catalog_isos;
CREATE TABLE catalog_isos (
	cluster TEXT NOT NULL,
	node    TEXT NOT NULL,
	storage TEXT NOT NULL,
	file    TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, node, storage, file)
);`

// schemaV18 adds a server-side CSRF token to each session. The cookie value
// is duplicated in the row so the middleware can validate the X-CSRF-Token
// header against both the cookie and the persisted session state.
const schemaV18 = `ALTER TABLE sessions ADD COLUMN csrf_token TEXT NOT NULL DEFAULT ''`

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
	{Version: 11, DDL: schemaV11},
	{Version: 12, DDL: schemaV12},
	{Version: 13, DDL: schemaV13},
	{Version: 14, DDL: schemaV14},
	{Version: 15, DDL: schemaV15},
	{Version: 16, DDL: schemaV16},
	{Version: 17, DDL: schemaV17},
	{Version: 18, DDL: schemaV18},
}
