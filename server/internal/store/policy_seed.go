package store

// schemaV10 creates the persistent T12 policy rows. The default row preserves
// the values that T04, T07, and T09 enforced before policy was persisted.
const schemaV10 = `
CREATE TABLE vm_limits (
	cluster             TEXT PRIMARY KEY,
	max_sockets         INTEGER NOT NULL,
	max_cores           INTEGER NOT NULL,
	max_memory_mb       INTEGER NOT NULL,
	max_disk_per_vm_gb  INTEGER NOT NULL,
	max_network_cards   INTEGER NOT NULL,
	max_snapshots       INTEGER NOT NULL,
	max_vm_per_user     INTEGER NOT NULL,
	allow_custom_yaml   BOOLEAN NOT NULL
);

INSERT INTO vm_limits (
	cluster, max_sockets, max_cores, max_memory_mb, max_disk_per_vm_gb,
	max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml
) VALUES ('default', 4, 8, 16384, 500, 4, 5, -1, 1);

CREATE TABLE node_limits (
	cluster      TEXT NOT NULL,
	node         TEXT NOT NULL,
	max_vms      INTEGER NOT NULL,
	max_vcpus    INTEGER NOT NULL,
	max_ram_gb   INTEGER NOT NULL,
	max_disk_gb  INTEGER NOT NULL,
	PRIMARY KEY (cluster, node)
);
`
