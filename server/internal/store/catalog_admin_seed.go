package store

// schemaV9 adds the admin-catalogue surface (T11): an enabled column to each
// of T06's five catalog tables (defaulted to 1 so every already-approved row
// stays approved with zero data migration — SC-003), plus the new catalog_tags
// table seeded with the mandatory, undeletable pvmss tag (FR-014).
//
// The enabled column changes what a row's presence means: before T11, presence
// meant "approved"; after T11, presence means "known to the admin surface" and
// the enabled column is what "approved" means. Row composite keys are
// unchanged from T06/T07.
const schemaV9 = `
ALTER TABLE catalog_nodes    ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE catalog_storages ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE catalog_bridges  ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE catalog_isos     ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE catalog_profiles ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;

CREATE TABLE catalog_tags (
	cluster     TEXT NOT NULL,
	name        TEXT NOT NULL,
	color       TEXT NOT NULL,
	created_at  TEXT NOT NULL,
	PRIMARY KEY (cluster, name)
);

INSERT INTO catalog_tags (cluster, name, color, created_at)
	VALUES ('default', 'pvmss', '#4f46e5', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
`
