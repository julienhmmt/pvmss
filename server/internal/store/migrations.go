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

// schemaV20 adds the audit retention configuration (issue #02). A single-row
// table seeded with the default 365-day retention; the floor of 30 days is
// enforced by SetAuditConfig, not by the schema, so a future floor change is
// a code edit rather than a migration.
const schemaV20 = `CREATE TABLE audit_config (
	id             INTEGER PRIMARY KEY CHECK (id = 1),
	retention_days INTEGER NOT NULL
);
INSERT INTO audit_config (id, retention_days) VALUES (1, 365);`

// schemaV21 adds a sockets column to catalog_profiles so the admin can set a
// profile's socket count (US2/D3b). Existing rows default to 1, preserving the
// previous hardcoded behaviour.
const schemaV21 = `ALTER TABLE catalog_profiles ADD COLUMN sockets INTEGER NOT NULL DEFAULT 1`

// schemaV22 adds the approved Proxmox template catalog (US2/issue-02): a
// sibling to catalog_isos, keyed by (cluster, vmid) since Proxmox VMIDs are
// cluster-unique. The node determines where the clone lands (D2b: cross-node
// clone is forbidden). cloud_init_capable drives the full/linked decision
// (D2c/issue-02 §5). disk_storage and disk_size_gb drive the resize decision
// (D2c: enlarge after clone, reject reduction before VMID).
//
// No seed: a real deployment starts with zero approved templates — admins
// approve whatever their cluster actually reports (the fake demo source
// discovers its own template set for demo mode).
const schemaV22 = `CREATE TABLE catalog_templates (
	cluster            TEXT NOT NULL,
	node               TEXT NOT NULL,
	vmid               INTEGER NOT NULL,
	name               TEXT NOT NULL DEFAULT '',
	cloud_init_capable BOOLEAN NOT NULL DEFAULT 0,
	disk_storage       TEXT NOT NULL DEFAULT '',
	disk_size_gb       INTEGER NOT NULL DEFAULT 0,
	disk_bus           TEXT NOT NULL DEFAULT 'scsi',
	enabled            BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, vmid)
);`

// schemaV23 adds an optional per-cluster isolation VLAN tag to vm_limits
// (US6/issue-06 D6b + Q18: one VLAN per cluster, imposed — the admin sets it
// alongside the gabarit; empty/0 = no tag imposed). Tenants never choose the
// segmentation; the create path stamps the tag on every NIC.
const schemaV23 = `ALTER TABLE vm_limits ADD COLUMN isolation_vlan_tag INTEGER NOT NULL DEFAULT 0`

// schemaV24 removes the demo template rows the V22 seed used to insert
// (debian-12-cloud and alpine-appliance on cluster "default"). Databases
// created before the seed was dropped must not keep offering templates that
// do not exist in Proxmox. The delete matches the exact seed signature so a
// legitimately approved template is never touched.
const schemaV24 = `DELETE FROM catalog_templates
	 WHERE cluster = 'default'
	   AND vmid IN (9000, 9001)
	   AND node = 'pve-node-02'
	   AND name IN ('debian-12-cloud', 'alpine-appliance')
	   AND disk_storage IN ('local-lvm', 'local')
	   AND disk_bus = 'scsi'`

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
	{Version: 19, DDL: schemaV19},
	{Version: 20, DDL: schemaV20},
	{Version: 21, DDL: schemaV21},
	{Version: 22, DDL: schemaV22},
	{Version: 23, DDL: schemaV23},
	{Version: 24, DDL: schemaV24},
}
