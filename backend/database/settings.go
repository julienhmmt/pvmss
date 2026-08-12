package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// VMLimits holds the resource-limit values stored in the vm_limits singleton row.
type VMLimits struct {
	MaxVMs          int  `json:"max_vms"`
	MaxVMPerUser    int  `json:"max_vm_per_user"`
	MaxNetworkCards int  `json:"max_network_cards"`
	MaxDiskPerVM    int  `json:"max_disk_per_vm"`
	AllowCustomYAML bool `json:"allow_custom_yaml"`
	MaxSnapshots    int  `json:"max_snapshots"`
}

// SFTPConfig holds the SSH/SFTP configuration stored in the sftp_config singleton row.
// PrivateKey, when set, holds the SSH private key content (encrypted at rest with
// the session secret); it takes precedence over PrivateKeyPath at connect time.
// HostKeyPath is the path to a known_hosts file used to verify the SSH server's
// host key; required when SFTP is enabled.
type SFTPConfig struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PrivateKeyPath string `json:"private_key_path"`
	PrivateKey     string `json:"private_key,omitempty"`
	RemotePath     string `json:"remote_path"`
	HostKeyPath    string `json:"host_key_path"`
}

// NodeLimit holds the per-node capacity caps stored in the node_limits table.
// Zero values mean "no cap" for that dimension.
type NodeLimit struct {
	NodeName  string `json:"node"`
	MaxVMs    int    `json:"max_vms"`
	MaxVCPUs  int    `json:"max_vcpus"`
	MaxRAMGB  int    `json:"max_ram_gb"`
	MaxDiskGB int    `json:"max_disk_gb"`
}

// AppSettings is the in-memory representation assembled from all DB tables.
// It is the type returned by LoadAppSettings and used to warm the StateManager cache.
type AppSettings struct {
	Limits             VMLimits             `json:"limits"`
	NodeLimits         map[string]NodeLimit `json:"node_limits"`
	EnabledNodes       []string             `json:"enabled_nodes"`
	EnabledStorages    []string             `json:"enabled_storages"`
	EnabledISOs        []string             `json:"enabled_isos"`
	EnabledVMBRs       []string             `json:"enabled_vmbrs"`
	Tags               []string             `json:"tags"`
	CloudInitTemplates []CloudInitTemplate  `json:"cloudinit_templates"`
	VMProfiles         []VMProfile          `json:"vm_profiles"`
	SFTPConfig         SFTPConfig           `json:"sftp_config"`
}

// GetVMLimits reads the singleton vm_limits row.
// Returns default values when no row exists yet (first run).
func (s *sqliteDB) GetVMLimits() (*VMLimits, error) {
	row := s.db.QueryRow(`
		SELECT max_vms, max_vm_per_user, max_network_cards,
		       max_disk_per_vm, allow_custom_yaml, max_snapshots
		FROM vm_limits WHERE id = 1
	`)
	lim, err := scanVMLimitsRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultVMLimits(), nil
	}
	return lim, err
}

// SetVMLimits upserts the singleton vm_limits row inside a transaction that
// also appends an audit entry with the before/after JSON snapshots.
// The audit action is "create" on first write and "update" thereafter.
func (s *sqliteDB) SetVMLimits(limits *VMLimits, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	row := tx.QueryRow(`
		SELECT max_vms, max_vm_per_user, max_network_cards,
		       max_disk_per_vm, allow_custom_yaml, max_snapshots
		FROM vm_limits WHERE id = 1
	`)
	oldLim, err := scanVMLimitsRow(row)
	action := "create"
	var oldJSON []byte
	if err == nil {
		action = "update"
		oldJSON, _ = json.Marshal(oldLim)
	} else if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return fmt.Errorf("read vm_limits for audit: %w", err)
	}
	newJSON, _ := json.Marshal(limits)
	if err := execUpsertVMLimits(tx, limits); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := appendAudit(tx, "vm_limits", "1", action, string(oldJSON), string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// execUpsertVMLimits executes the vm_limits UPSERT within tx.
func execUpsertVMLimits(tx *sql.Tx, l *VMLimits) error {
	_, err := tx.Exec(`
		INSERT INTO vm_limits
		    (id, max_vms, max_vm_per_user, max_network_cards,
		     max_disk_per_vm, allow_custom_yaml, max_snapshots, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
		    max_vms           = excluded.max_vms,
		    max_vm_per_user   = excluded.max_vm_per_user,
		    max_network_cards = excluded.max_network_cards,
		    max_disk_per_vm   = excluded.max_disk_per_vm,
		    allow_custom_yaml = excluded.allow_custom_yaml,
		    max_snapshots     = excluded.max_snapshots,
		    updated_at        = CURRENT_TIMESTAMP
	`, l.MaxVMs, l.MaxVMPerUser, l.MaxNetworkCards, l.MaxDiskPerVM,
		boolToInt(l.AllowCustomYAML), l.MaxSnapshots)
	return err
}

// scanVMLimitsRow scans one vm_limits row from a *sql.Row.
func scanVMLimitsRow(row *sql.Row) (*VMLimits, error) {
	var l VMLimits
	var allowCustomYAML int
	if err := row.Scan(&l.MaxVMs, &l.MaxVMPerUser, &l.MaxNetworkCards,
		&l.MaxDiskPerVM, &allowCustomYAML, &l.MaxSnapshots); err != nil {
		return nil, err
	}
	l.AllowCustomYAML = allowCustomYAML != 0
	return &l, nil
}

// defaultVMLimits returns defaults aligned with state.defaultSettings().
// These values must stay in sync with the constants in state/settings.go.
func defaultVMLimits() *VMLimits {
	return &VMLimits{
		MaxVMs:          0, // no global VM cap in state defaults
		MaxVMPerUser:    5, // state.DefaultVMPerUser
		MaxNetworkCards: 1, // state.MinNetworkCards
		MaxDiskPerVM:    4, // state.DefaultDiskPerVM
		AllowCustomYAML: true,
		MaxSnapshots:    8, // state.DefaultSnapshotsPerVM
	}
}

// boolToInt converts a bool to 0/1 for SQLite BOOLEAN columns.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetNodeLimits returns a map of node name → NodeLimit with all capacity caps.
func (s *sqliteDB) GetNodeLimits() (map[string]NodeLimit, error) {
	rows, err := s.db.Query(`
		SELECT node_name, max_vms, max_vcpus, max_ram_gb, max_disk_gb
		FROM node_limits
	`)
	if err != nil {
		return nil, fmt.Errorf("query node_limits: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeLimitsRows(rows)
}

// scanNodeLimitsRows scans node_limits rows into a map.
func scanNodeLimitsRows(rows *sql.Rows) (map[string]NodeLimit, error) {
	result := make(map[string]NodeLimit)
	for rows.Next() {
		var nl NodeLimit
		if err := rows.Scan(&nl.NodeName, &nl.MaxVMs, &nl.MaxVCPUs, &nl.MaxRAMGB, &nl.MaxDiskGB); err != nil {
			return nil, fmt.Errorf("scan node_limits: %w", err)
		}
		result[nl.NodeName] = nl
	}
	return result, rows.Err()
}

// SetNodeLimit upserts all capacity limits for a single node.
func (s *sqliteDB) SetNodeLimit(limit NodeLimit, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	rows, err := tx.Query(`
		SELECT node_name, max_vms, max_vcpus, max_ram_gb, max_disk_gb FROM node_limits
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("query node_limits: %w", err)
	}
	oldMap, err := scanNodeLimitsRows(rows)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	action := auditAction(oldMap, limit.NodeName)
	oldJSON, _ := json.Marshal(oldMap[limit.NodeName])
	newJSON, _ := json.Marshal(limit)
	_, execErr := tx.Exec(`
		INSERT INTO node_limits (node_name, max_vms, max_vcpus, max_ram_gb, max_disk_gb, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(node_name) DO UPDATE SET
		    max_vms    = excluded.max_vms,
		    max_vcpus  = excluded.max_vcpus,
		    max_ram_gb = excluded.max_ram_gb,
		    max_disk_gb = excluded.max_disk_gb,
		    updated_at = CURRENT_TIMESTAMP
	`, limit.NodeName, limit.MaxVMs, limit.MaxVCPUs, limit.MaxRAMGB, limit.MaxDiskGB)
	if execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert node_limits: %w", execErr)
	}
	if err := appendAudit(tx, "node_limits", limit.NodeName, action, string(oldJSON), string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteNodeLimit removes the per-node limit override for node.
// Returns ErrNotFound when no limit exists for the given node.
func (s *sqliteDB) DeleteNodeLimit(node string, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	rows, err := tx.Query(`
		SELECT node_name, max_vms, max_vcpus, max_ram_gb, max_disk_gb FROM node_limits
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("query node_limits: %w", err)
	}
	oldMap, err := scanNodeLimitsRows(rows)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, exists := oldMap[node]; !exists {
		_ = tx.Rollback()
		return fmt.Errorf("delete node_limit %q: %w", node, ErrNotFound)
	}
	oldJSON, _ := json.Marshal(oldMap[node])
	if _, execErr := tx.Exec(`DELETE FROM node_limits WHERE node_name = ?`, node); execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete node_limits: %w", execErr)
	}
	if err := appendAudit(tx, "node_limits", node, "delete", string(oldJSON), "", changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// auditAction returns "create" when key is not present in m, otherwise "update".
func auditAction[K comparable, V any](m map[K]V, key K) string {
	if _, exists := m[key]; exists {
		return "update"
	}
	return "create"
}

// GetSFTPConfig reads the singleton sftp_config row.
// Returns safe defaults when no row exists yet.
func (s *sqliteDB) GetSFTPConfig() (*SFTPConfig, error) {
	row := s.db.QueryRow(`
		SELECT enabled,
		       COALESCE(host,''), port, COALESCE(username,''),
		       COALESCE(private_key_path,''), COALESCE(remote_path,''),
		       COALESCE(private_key,''), COALESCE(host_key_path,'')
		FROM sftp_config WHERE id = 1
	`)
	cfg, err := scanSFTPConfigRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultSFTPConfig(), nil
	}
	return cfg, err
}

// SetSFTPConfig upserts the singleton sftp_config row inside a transaction
// that also appends an audit entry.
// The audit action is "create" on first write and "update" thereafter.
// The PrivateKey (encrypted at rest) is redacted from the audit snapshots so
// the ciphertext is never duplicated into the audit_log table.
func (s *sqliteDB) SetSFTPConfig(cfg *SFTPConfig, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	row := tx.QueryRow(`
		SELECT enabled,
		       COALESCE(host,''), port, COALESCE(username,''),
		       COALESCE(private_key_path,''), COALESCE(remote_path,''),
		       COALESCE(private_key,''), COALESCE(host_key_path,'')
		FROM sftp_config WHERE id = 1
	`)
	oldCfg, err := scanSFTPConfigRow(row)
	action := "create"
	var oldJSON []byte
	if err == nil {
		action = "update"
		oldJSON, _ = json.Marshal(redactSFTPPrivateKeyForAudit(oldCfg))
	} else if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return fmt.Errorf("read sftp_config for audit: %w", err)
	}
	newJSON, _ := json.Marshal(redactSFTPPrivateKeyForAudit(cfg))
	if err := execUpsertSFTPConfig(tx, cfg); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := appendAudit(tx, "sftp_config", "1", action, string(oldJSON), string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// redactSFTPPrivateKeyForAudit returns a shallow copy of cfg with PrivateKey
// replaced by a sentinel, so the encrypted ciphertext never lands in the
// audit_log. The sentinel preserves auditability: "[set]" when a key exists,
// "" when none is stored.
func redactSFTPPrivateKeyForAudit(cfg *SFTPConfig) SFTPConfig {
	out := *cfg
	if out.PrivateKey != "" {
		out.PrivateKey = "[set]"
	}
	return out
}

// execUpsertSFTPConfig executes the sftp_config UPSERT within tx.
func execUpsertSFTPConfig(tx *sql.Tx, c *SFTPConfig) error {
	_, err := tx.Exec(`
		INSERT INTO sftp_config
		    (id, enabled, host, port, username, private_key_path, remote_path, private_key, host_key_path, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
		    enabled          = excluded.enabled,
		    host             = excluded.host,
		    port             = excluded.port,
		    username         = excluded.username,
		    private_key_path = excluded.private_key_path,
		    remote_path      = excluded.remote_path,
		    private_key      = excluded.private_key,
		    host_key_path    = excluded.host_key_path,
		    updated_at       = CURRENT_TIMESTAMP
	`, boolToInt(c.Enabled), c.Host, c.Port, c.Username, c.PrivateKeyPath, c.RemotePath, c.PrivateKey, c.HostKeyPath)
	return err
}

// scanSFTPConfigRow scans one sftp_config row from a *sql.Row.
func scanSFTPConfigRow(row *sql.Row) (*SFTPConfig, error) {
	var c SFTPConfig
	var enabled int
	if err := row.Scan(&enabled, &c.Host, &c.Port, &c.Username, &c.PrivateKeyPath, &c.RemotePath, &c.PrivateKey, &c.HostKeyPath); err != nil {
		return nil, err
	}
	c.Enabled = enabled != 0
	return &c, nil
}

// defaultSFTPConfig returns sane SFTP defaults.
func defaultSFTPConfig() *SFTPConfig {
	return &SFTPConfig{
		Enabled:        false,
		Port:           22,
		Username:       "pvmss-snippets",
		PrivateKeyPath: "/app/pvmss_snippets_ed25519",
		RemotePath:     "/var/lib/vz/snippets",
		HostKeyPath:    "/app/pvmss_known_hosts",
	}
}

// LoadAppSettings assembles a complete AppSettings snapshot from all DB tables
// in a sequence of read queries. Used to warm the StateManager in-memory cache.
func (s *sqliteDB) LoadAppSettings() (*AppSettings, error) {
	limits, err := s.GetVMLimits()
	if err != nil {
		return nil, fmt.Errorf("load vm_limits: %w", err)
	}
	nodeLimits, err := s.GetNodeLimits()
	if err != nil {
		return nil, fmt.Errorf("load node_limits: %w", err)
	}
	nodes, err := s.GetEnabledNodes()
	if err != nil {
		return nil, fmt.Errorf("load enabled_nodes: %w", err)
	}
	storages, err := s.GetEnabledStorages()
	if err != nil {
		return nil, fmt.Errorf("load enabled_storages: %w", err)
	}
	isos, err := s.GetEnabledISOs()
	if err != nil {
		return nil, fmt.Errorf("load enabled_isos: %w", err)
	}
	vmbrs, err := s.GetEnabledVMBRs()
	if err != nil {
		return nil, fmt.Errorf("load enabled_vmbrs: %w", err)
	}
	tags, err := s.GetTags()
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}
	templates, err := s.ListCloudInitTemplates()
	if err != nil {
		return nil, fmt.Errorf("load cloudinit_templates: %w", err)
	}
	profiles, err := s.ListVMProfiles()
	if err != nil {
		return nil, fmt.Errorf("load vm_profiles: %w", err)
	}
	sftp, err := s.GetSFTPConfig()
	if err != nil {
		return nil, fmt.Errorf("load sftp_config: %w", err)
	}
	return &AppSettings{
		Limits:             *limits,
		NodeLimits:         nodeLimits,
		EnabledNodes:       nodes,
		EnabledStorages:    storages,
		EnabledISOs:        isos,
		EnabledVMBRs:       vmbrs,
		Tags:               tags,
		CloudInitTemplates: templates,
		VMProfiles:         profiles,
		SFTPConfig:         *sftp,
	}, nil
}
