package database

// schemaV1 contains all CREATE TABLE and CREATE INDEX statements for schema version 1.
// Every statement uses IF NOT EXISTS so the block is idempotent when re-applied.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS vm_limits (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    max_vms          INTEGER NOT NULL DEFAULT 10,
    max_vm_per_user  INTEGER NOT NULL DEFAULT 2,
    max_network_cards INTEGER NOT NULL DEFAULT 2,
    max_disk_per_vm  INTEGER NOT NULL DEFAULT 4,
    allow_custom_yaml BOOLEAN NOT NULL DEFAULT 0,
    max_snapshots    INTEGER NOT NULL DEFAULT 3,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS node_limits (
    node_name  TEXT PRIMARY KEY,
    max_vms    INTEGER,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enabled_nodes (
    name       TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enabled_storages (
    storage_id TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enabled_isos (
    name       TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enabled_vmbrs (
    name       TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    name       TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cloudinit_templates (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT,
    storage      TEXT,
    filename     TEXT,
    yaml_content TEXT NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT 1,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS vm_profiles (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    config      TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT 1,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sftp_config (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    enabled          BOOLEAN NOT NULL DEFAULT 0,
    host             TEXT,
    port             INTEGER DEFAULT 22,
    username         TEXT,
    private_key_path TEXT,
    remote_path      TEXT,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    record_id  TEXT NOT NULL,
    action     TEXT NOT NULL,
    old_value  TEXT,
    new_value  TEXT,
    changed_by TEXT NOT NULL,
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_table_record ON audit_log(table_name, record_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_changed_at   ON audit_log(changed_at);

CREATE TABLE IF NOT EXISTS app_bootstrap (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    completed    BOOLEAN NOT NULL DEFAULT 0,
    completed_at TIMESTAMP,
    version      TEXT
);
`

// schemaV2 adds capacity-limit columns to the node_limits table.
// SQLite's ALTER TABLE only supports adding columns (no dropping/renaming in older versions),
// so this migration is forward-only and fully idempotent via IF NOT EXISTS semantics.
const schemaV2 = `
ALTER TABLE node_limits ADD COLUMN max_vcpus  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_limits ADD COLUMN max_ram_gb INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_limits ADD COLUMN max_disk_gb INTEGER NOT NULL DEFAULT 0;
`

// schemaV3 adds a column to store the SFTP private key content (encrypted at
// rest) so the key can be managed from the web UI instead of a mounted file.
const schemaV3 = `
ALTER TABLE sftp_config ADD COLUMN private_key TEXT;
`

// schemaV4 adds a column to store the path to a known_hosts file used to
// verify the Proxmox SSH/SFTP server's host key. Without this, the SFTP
// client used InsecureIgnoreHostKey, which disabled MITM protection.
const schemaV4 = `
ALTER TABLE sftp_config ADD COLUMN host_key_path TEXT;
`
