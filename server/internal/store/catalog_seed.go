package store

// schemaV7 adds the approved-resource catalog (T06, AC03 §1.2): the nodes,
// storages, bridges, ISOs, and VM profiles a creation request may reference.
// Every table carries a cluster column (constitution V) even though only one
// cluster ("default") is configured today.
//
// The rows below are hand-authored fixture data (T06 Assumptions): T11's
// admin CRUD will make them editable, T06 only consumes them. The fixture
// deliberately does NOT approve all of T01's fake dataset — a catalog that
// approves everything would never exercise FR-003's rejection path.
const schemaV7 = `
CREATE TABLE catalog_nodes (
	cluster TEXT NOT NULL,
	name    TEXT NOT NULL,
	PRIMARY KEY (cluster, name)
);
CREATE TABLE catalog_storages (
	cluster TEXT NOT NULL,
	name    TEXT NOT NULL,
	node    TEXT NOT NULL,
	PRIMARY KEY (cluster, name, node)
);
CREATE TABLE catalog_bridges (
	cluster TEXT NOT NULL,
	node    TEXT NOT NULL,
	name    TEXT NOT NULL,
	PRIMARY KEY (cluster, node, name)
);
CREATE TABLE catalog_isos (
	cluster TEXT NOT NULL,
	storage TEXT NOT NULL,
	file    TEXT NOT NULL,
	PRIMARY KEY (cluster, storage, file)
);
CREATE TABLE catalog_profiles (
	cluster   TEXT NOT NULL,
	id        TEXT NOT NULL,
	label     TEXT NOT NULL,
	cpu_cores INTEGER NOT NULL,
	memory_mb INTEGER NOT NULL,
	disk_gb   INTEGER NOT NULL,
	bus       TEXT NOT NULL,
	PRIMARY KEY (cluster, id)
);

INSERT INTO catalog_nodes (cluster, name) VALUES
	('default', 'pve-node-01'),
	('default', 'pve-node-02');

INSERT INTO catalog_storages (cluster, name, node) VALUES
	('default', 'local-lvm', 'pve-node-01'),
	('default', 'local', 'pve-node-02'),
	('default', 'ceph-data', 'pve-node-02');

INSERT INTO catalog_bridges (cluster, node, name) VALUES
	('default', 'pve-node-01', 'vmbr0'),
	('default', 'pve-node-01', 'vmbr1');

INSERT INTO catalog_isos (cluster, storage, file) VALUES
	('default', 'local', 'debian-12-generic-amd64.iso'),
	('default', 'local', 'ubuntu-24.04-server-amd64.iso');

INSERT INTO catalog_profiles (cluster, id, label, cpu_cores, memory_mb, disk_gb, bus) VALUES
	('default', 'small', 'Small (1 vCPU, 2 GB, 20 GB)', 1, 2048, 20, 'scsi'),
	('default', 'medium', 'Medium (2 vCPU, 4 GB, 40 GB)', 2, 4096, 40, 'scsi'),
	('default', 'large', 'Large (4 vCPU, 8 GB, 80 GB)', 4, 8192, 80, 'scsi');
`
