// Package store provides SQLite-backed persistence and schema migrations.
package store

// schemaV1 is the v0.4 baseline: the full schema in its final form. The
// incremental chain accumulated during development (37 versions) was
// collapsed before the first tagged release - no v0.4 database exists in
// the field to upgrade. Databases created by pre-release builds carry
// schema_migrations versions that are absent from this list; RunMigrations
// rejects them with a clear error instead of silently skipping the baseline.
//
// Only the structural audit_config singleton is seeded here. Everything else
// is runtime-seeded: vm_limits by CreateCluster, the pvmss tag lazily by
// catalog.ListTags, documentation_pages by docs/seed at startup, and the demo
// catalog by SeedDemoCatalog when the cluster source is "fake".
const schemaV1 = `
CREATE TABLE sessions (
	token_hash BLOB PRIMARY KEY,
	username   TEXT NOT NULL,
	is_admin   INTEGER NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	pool       TEXT NOT NULL DEFAULT '',
	cluster    TEXT NOT NULL DEFAULT '',
	csrf_token TEXT NOT NULL DEFAULT ''
);

-- vmid is nullable: admin actions (cluster credentials, policy, catalog
-- toggles, db import/export) have no VM scope but are still recorded.
CREATE TABLE audit_log (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	actor       TEXT NOT NULL,
	cluster     TEXT NOT NULL,
	vmid        INTEGER,
	action      TEXT NOT NULL,
	timestamp   TEXT NOT NULL,
	target_type TEXT,
	target_id   TEXT,
	detail      TEXT,
	ip_address  TEXT,
	severity    TEXT NOT NULL DEFAULT 'info'
);

CREATE TABLE clusters (
	name                     TEXT PRIMARY KEY,
	url                      TEXT NOT NULL,
	tls_insecure_skip_verify INTEGER NOT NULL DEFAULT 0,
	token_id                 TEXT NOT NULL,
	token_secret_ciphertext  BLOB,
	created_at               TEXT NOT NULL,
	removed_at               TEXT,
	last_test_status         TEXT,
	last_test_at             TEXT,
	last_test_message        TEXT,
	proxmox_version          TEXT,
	display_name             TEXT,
	snippet_storage          TEXT NOT NULL DEFAULT ''
);

CREATE TABLE catalog_nodes (
	cluster TEXT NOT NULL,
	name    TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, name)
);

CREATE TABLE catalog_storages (
	cluster TEXT NOT NULL,
	name    TEXT NOT NULL,
	node    TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, name, node)
);

CREATE TABLE catalog_bridges (
	cluster TEXT NOT NULL,
	node    TEXT NOT NULL,
	name    TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, node, name)
);

CREATE TABLE catalog_isos (
	cluster TEXT NOT NULL,
	node    TEXT NOT NULL,
	storage TEXT NOT NULL,
	file    TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, node, storage, file)
);

-- Approved cloud images: bootable disk images an admin placed on a Proxmox
-- storage's import/ directory - PVMSS never fetches images from the
-- internet. size_bytes lets the create path reject a disk size below the
-- image before a VMID is spent.
CREATE TABLE catalog_images (
	cluster    TEXT NOT NULL,
	node       TEXT NOT NULL,
	storage    TEXT NOT NULL,
	file       TEXT NOT NULL,
	size_bytes INTEGER NOT NULL DEFAULT 0,
	enabled    BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, node, storage, file)
);

-- Approved Proxmox templates, keyed by (cluster, vmid) since VMIDs are
-- cluster-unique. cloud_init_capable drives the full/linked clone decision;
-- override_discovery pins the stored fields against discovery write-back.
CREATE TABLE catalog_templates (
	cluster            TEXT NOT NULL,
	node               TEXT NOT NULL,
	vmid               INTEGER NOT NULL,
	name               TEXT NOT NULL DEFAULT '',
	cloud_init_capable BOOLEAN NOT NULL DEFAULT 0,
	disk_storage       TEXT NOT NULL DEFAULT '',
	disk_size_gb       INTEGER NOT NULL DEFAULT 0,
	disk_bus           TEXT NOT NULL DEFAULT 'scsi',
	enabled            BOOLEAN NOT NULL DEFAULT 1,
	override_discovery BOOLEAN NOT NULL DEFAULT 0,
	PRIMARY KEY (cluster, vmid)
);

CREATE TABLE catalog_profiles (
	cluster   TEXT NOT NULL,
	id        TEXT NOT NULL,
	label     TEXT NOT NULL,
	cpu_cores INTEGER NOT NULL,
	memory_mb INTEGER NOT NULL,
	disk_gb   INTEGER NOT NULL,
	bus       TEXT NOT NULL,
	enabled   BOOLEAN NOT NULL DEFAULT 1,
	sockets   INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY (cluster, id)
);

CREATE TABLE catalog_tags (
	cluster    TEXT NOT NULL,
	name       TEXT NOT NULL,
	color      TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (cluster, name)
);

CREATE TABLE catalog_cloudinit_templates (
	cluster    TEXT NOT NULL,
	id         TEXT NOT NULL,
	label      TEXT NOT NULL,
	content    TEXT NOT NULL,
	enabled    BOOLEAN NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (cluster, id)
);

CREATE TABLE documentation_pages (
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
);

-- Pools PVMSS provisioned, so deletion can be scoped to managed pools only.
-- A row is written only after pools.Create succeeds end-to-end; pre-existing
-- Proxmox pools are intentionally not adopted.
CREATE TABLE managed_pools (
	cluster    TEXT NOT NULL,
	name       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (cluster, name)
);

CREATE TABLE vm_limits (
	cluster            TEXT PRIMARY KEY,
	max_sockets        INTEGER NOT NULL,
	max_cores          INTEGER NOT NULL,
	max_memory_mb      INTEGER NOT NULL,
	max_disk_per_vm_gb INTEGER NOT NULL,
	max_network_cards  INTEGER NOT NULL,
	max_snapshots      INTEGER NOT NULL,
	max_vm_per_user    INTEGER NOT NULL,
	isolation_vlan_tag INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE node_limits (
	cluster     TEXT NOT NULL,
	node        TEXT NOT NULL,
	max_vms     INTEGER NOT NULL,
	max_vcpus   INTEGER NOT NULL,
	max_ram_gb  INTEGER NOT NULL,
	max_disk_gb INTEGER NOT NULL,
	PRIMARY KEY (cluster, node)
);

CREATE TABLE audit_config (
	id             INTEGER PRIMARY KEY CHECK (id = 1),
	retention_days INTEGER NOT NULL
);
INSERT INTO audit_config (id, retention_days) VALUES (1, 365);

-- Per-VM baseline delivery state for image-mode VMs. state is "applied",
-- "override", or "not_delivered"; error carries the reason when
-- state = 'not_delivered'.
CREATE TABLE vm_baseline_state (
	cluster   TEXT NOT NULL,
	vmid      INTEGER NOT NULL,
	state     TEXT NOT NULL,
	error     TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (cluster, vmid)
);

-- Which admin-published cloud-init file a VM uses. Unlike the legacy
-- vm_cloudinit_snippets rows, its file is never deleted with the VM.
CREATE TABLE vm_cloudinit_documents (
	cluster     TEXT NOT NULL,
	vmid        INTEGER NOT NULL,
	template_id TEXT NOT NULL,
	filename    TEXT NOT NULL,
	updated_at  TEXT NOT NULL,
	updated_by  TEXT NOT NULL,
	PRIMARY KEY (cluster, vmid)
);

-- Legacy per-VM cloud-init files. Forgotten, never deleted: the rows record
-- snippets that exist on Proxmox storage and PVMSS cannot remove.
CREATE TABLE vm_cloudinit_snippets (
	cluster    TEXT NOT NULL,
	vmid       INTEGER NOT NULL,
	content    TEXT NOT NULL,
	storage    TEXT NOT NULL,
	filename   TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	PRIMARY KEY (cluster, vmid)
);

CREATE TABLE profile_ssh_keys (
	id         TEXT PRIMARY KEY,
	cluster    TEXT NOT NULL,
	username   TEXT NOT NULL,
	label      TEXT NOT NULL,
	public_key TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE (cluster, username, label),
	UNIQUE (cluster, username, public_key)
);
`

// Migration is a single schema version and its forward-only DDL.
type Migration struct {
	Version int
	DDL     string
}

// Migrations is the ordered list of schema migrations.
// Versions must be consecutive integers starting at 1.
var Migrations = []Migration{
	{Version: 1, DDL: schemaV1},
}
