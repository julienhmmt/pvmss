// Package recovery implements the one-time v0.3 → v0.4 data migration tool
// (T16 — Bascule). It reads a legacy SQLite database and upserts
// admin-configured facts into a fresh v0.4 database. The package never
// touches sftp_config (FR-004) and never requires a running v0.4 server
// process (FR-011).
package recovery

import "context"

// Summary is the recovery run's per-table output. It is printed to stdout
// for the operator to review before proceeding to the parity checklist.
type Summary struct {
	Cluster         TableResult
	CatalogNodes    TableResult
	CatalogStorages TableResult
	CatalogBridges  TableResult
	CatalogISOs     TableResult
	CatalogProfiles TableResult
	CatalogTags     TableResult
	VMLimits        TableResult
	NodeLimits      TableResult
}

// TableResult tracks per-table read/written/skipped counts and the reasons
// for every skipped row. A non-zero Skipped with zero errors is a normal,
// expected result for a real legacy database (spec Edge Cases).
type TableResult struct {
	Read        int
	Written     int
	Skipped     int
	SkipReasons []string
	// Note is an optional human-readable annotation rendered after the
	// counts (e.g. the vm_limits "left at shipped defaults" note).
	Note string
}

// ProxmoxCreds holds the cluster connection credentials derived from
// environment variables or flag overrides. When TokenSecret is empty,
// storage-node expansion is skipped (FR-011).
type ProxmoxCreds struct {
	URL         string
	TokenID     string
	TokenSecret string
}

// Environ abstracts environment variable access so Run can be tested
// without mutating the real process environment.
type Environ interface {
	Get(key string) string
}

// osEnviron adapts the real os.Getenv to the Environ interface.
type osEnviron struct{}

func (osEnviron) Get(key string) string { return getEnv(key) }

// StorageNodeResolver returns the list of nodes that report a given storage
// name in live cluster discovery. If no Proxmox connection is available the
// caller passes nil and every storage is skipped with a named reason.
// This is the one interface in this tranche that touches live Proxmox
// (FR-011, data-model.md §1).
type StorageNodeResolver interface {
	StorageNodes(ctx context.Context, storageName string) ([]string, error)
}

// --- Row types returned by the per-table mapping functions ---

// ClusterRow is the single clusters row the tool writes per invocation.
// TokenSecretCiphertext is the AES-256-GCM-encrypted token secret.
type ClusterRow struct {
	Name                  string
	URL                   string
	TLSInsecureSkipVerify bool
	TokenID               string
	TokenSecretCiphertext []byte
	OIDCEnabled           bool
	CreatedAt             string
}

// NodeRow is one enabled_nodes → catalog_nodes mapping.
type NodeRow struct {
	Name    string
	Enabled bool
}

// BridgeRow is one enabled_vmbrs → catalog_bridges mapping.
type BridgeRow struct {
	Name    string
	Node    string
	Enabled bool
}

// ISORow is one enabled_isos → catalog_isos mapping after volid splitting.
// Node is empty because the legacy table did not record per-node discovery;
// the v0.4 schema stores node="" for these legacy entries.
type ISORow struct {
	Storage string
	File    string
	Enabled bool
}

// StorageRow is one enabled_storages → catalog_storages mapping after
// live node expansion. Node is empty when expansion was not performed.
type StorageRow struct {
	Name    string
	Node    string
	Enabled bool
}

// ProfileRow is one vm_profiles → catalog_profiles mapping after JSON parse.
type ProfileRow struct {
	ID       string
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
	Enabled  bool
}

// TagRow is one tags → catalog_tags mapping with an assigned default color.
type TagRow struct {
	Name      string
	Color     string
	CreatedAt string
}

// VMLimitsRow carries the five vm_limits fields legacy actually persisted.
// max_sockets/max_cores/max_memory_mb are intentionally absent — there is
// no on-disk source for them (FR-003, SC-002).
type VMLimitsRow struct {
	MaxDiskPerVMGB  int
	MaxNetworkCards int
	MaxSnapshots    int
	MaxVMPerUser    int
	AllowCustomYAML bool
}

// NodeLimitsRow is one node_limits → node_limits mapping.
type NodeLimitsRow struct {
	Node      string
	MaxVMs    int
	MaxVCPUs  int
	MaxRAMGB  int
	MaxDiskGB int
}

// SkipReason pairs a row identifier with the reason it was skipped.
type SkipReason struct {
	Row    string
	Reason string
}
