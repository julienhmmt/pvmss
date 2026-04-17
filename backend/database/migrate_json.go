// Package database – JSON-to-SQLite migration.
//
// MigrateFromJSON reads a legacy settings.json snapshot and writes all
// settings into the SQLite database within a single transaction.
// On any failure the transaction is rolled back and the database
// is left unchanged.
package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

// ── Input types ───────────────────────────────────────────────────────────────

// JSONSettings mirrors state.AppSettings for settings.json deserialization.
// Defined here to avoid an import cycle (the state package will eventually
// depend on database).
type JSONSettings struct {
	EnabledNodes       []string                `json:"enabled_nodes"`
	EnabledStorages    []string                `json:"enabled_storages"`
	ISOs               []string                `json:"isos"`
	VMBRs              []string                `json:"vmbrs"`
	Tags               []string                `json:"tags"`
	Limits             JSONLimitsConfig        `json:"limits"`
	MaxNetworkCards    int                     `json:"max_network_cards"`
	MaxDiskPerVM       int                     `json:"max_disk_per_vm"`
	MaxVMPerUser       int                     `json:"max_vm_per_user"`
	AllowCustomYAML    bool                    `json:"allow_custom_yaml"`
	CloudInitTemplates []JSONCloudInitTemplate `json:"cloudinit_templates"`
	VMProfiles         []JSONVMProfileConfig   `json:"vm_profiles"`
	CloudInitSFTP      JSONSFTPConfig          `json:"cloudinit_sftp"`
}

// JSONLimitsConfig is the limits subset from settings.json.
type JSONLimitsConfig struct {
	MaxSnapshots int `json:"max_snapshots"`
}

// JSONCloudInitTemplate mirrors state.CloudInitTemplate.
type JSONCloudInitTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Storage     string `json:"storage"`
	Filename    string `json:"filename"`
	YAMLContent string `json:"yaml_content"`
	Enabled     bool   `json:"enabled"`
}

// JSONVMProfileConfig mirrors state.VMProfileConfig.
type JSONVMProfileConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sockets     int    `json:"sockets"`
	Cores       int    `json:"cores"`
	RAMGB       int    `json:"ram_gb"`
	DiskGB      int    `json:"disk_gb"`
	DiskBus     string `json:"disk_bus"`
	Node        string `json:"node,omitempty"`
	Storage     string `json:"storage,omitempty"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Enabled     bool   `json:"enabled"`
}

// JSONSFTPConfig mirrors proxmox.CloudInitSFTPConfig.
// SnippetBaseDir maps to SFTPConfig.RemotePath in the database.
type JSONSFTPConfig struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PrivateKeyPath string `json:"private_key_path"`
	SnippetBaseDir string `json:"snippet_base_dir"`
}

// VMProfileConfigBlob is the JSON blob stored in vm_profiles.config.
// Shared by database, state, and api/v1 packages to avoid duplication.
type VMProfileConfigBlob struct {
	Sockets int    `json:"sockets"`
	Cores   int    `json:"cores"`
	RAMGB   int    `json:"ram_gb"`
	DiskGB  int    `json:"disk_gb"`
	DiskBus string `json:"disk_bus"`
	Node    string `json:"node,omitempty"`
	Storage string `json:"storage,omitempty"`
	Icon    string `json:"icon"`
	Color   string `json:"color"`
}

// vmProfileConfigData is an alias kept for backward compatibility within
// this package during the migration path.
type vmProfileConfigData = VMProfileConfigBlob

// ── Output type ───────────────────────────────────────────────────────────────

// MigrationSummary reports record counts written per table during a migration.
type MigrationSummary struct {
	NodesCount      int `json:"nodes_count"`
	StoragesCount   int `json:"storages_count"`
	ISOsCount       int `json:"isos_count"`
	VMBRsCount      int `json:"vmbrs_count"`
	TagsCount       int `json:"tags_count"`
	CloudInitCount  int `json:"cloudinit_count"`
	VMProfilesCount int `json:"vm_profiles_count"`
}

// ── Public API ────────────────────────────────────────────────────────────────

// ReadJSONSettings parses the settings.json file at path into a JSONSettings.
func ReadJSONSettings(path string) (*JSONSettings, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s JSONSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

// MigrateFromJSON migrates all settings from src into db using a single
// transaction.  changedBy is recorded as the actor in every audit_log row.
// On any failure the transaction is rolled back and db is left unchanged.
func MigrateFromJSON(db DB, src *JSONSettings, changedBy string) (*MigrationSummary, error) {
	impl, ok := db.(*sqliteDB)
	if !ok {
		return nil, fmt.Errorf("MigrateFromJSON: unsupported DB implementation")
	}
	summary := &MigrationSummary{}
	tx, err := impl.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin migration: %w", err)
	}
	if err := migrateAllTables(tx, src, changedBy, summary); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := appendMigrationAudit(tx, summary, changedBy); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := markBootstrapCompleteTx(tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit migration: %w", err)
	}
	if err := validateMigration(impl, summary); err != nil {
		return summary, fmt.Errorf("post-migration validation: %w", err)
	}
	return summary, nil
}

// ── Internal orchestration ────────────────────────────────────────────────────

func migrateAllTables(tx *sql.Tx, src *JSONSettings, changedBy string, summary *MigrationSummary) error {
	if err := migrateVMLimitsTx(tx, src, changedBy); err != nil {
		return err
	}
	if err := migrateListsTx(tx, src, changedBy, summary); err != nil {
		return err
	}
	if err := migrateCloudInitTemplatesTx(tx, src.CloudInitTemplates, changedBy, summary); err != nil {
		return err
	}
	if err := migrateVMProfilesTx(tx, src.VMProfiles, changedBy, summary); err != nil {
		return err
	}
	return migrateSFTPConfigTx(tx, src.CloudInitSFTP, changedBy)
}

// ── Per-table migration helpers ───────────────────────────────────────────────

func migrateVMLimitsTx(tx *sql.Tx, src *JSONSettings, changedBy string) error {
	lim := jsonToVMLimits(src)
	newJSON, _ := json.Marshal(lim)
	if err := execUpsertVMLimits(tx, lim); err != nil {
		return fmt.Errorf("upsert vm_limits: %w", err)
	}
	return appendAudit(tx, "vm_limits", "1", "create", "", string(newJSON), changedBy)
}

// listSpec describes a single list-table migration step.
type listSpec struct {
	table     string
	items     []string
	deleteSQL string
	insertSQL string
	countDst  *int
}

func migrateListsTx(tx *sql.Tx, src *JSONSettings, changedBy string, summary *MigrationSummary) error {
	specs := []listSpec{
		{"enabled_nodes", coalesceSlice(src.EnabledNodes),
			`DELETE FROM enabled_nodes`,
			`INSERT INTO enabled_nodes (name, enabled) VALUES (?, 1)`,
			&summary.NodesCount},
		{"enabled_storages", coalesceSlice(src.EnabledStorages),
			`DELETE FROM enabled_storages`,
			`INSERT INTO enabled_storages (storage_id, enabled) VALUES (?, 1)`,
			&summary.StoragesCount},
		{"enabled_isos", coalesceSlice(src.ISOs),
			`DELETE FROM enabled_isos`,
			`INSERT INTO enabled_isos (name, enabled) VALUES (?, 1)`,
			&summary.ISOsCount},
		{"enabled_vmbrs", coalesceSlice(src.VMBRs),
			`DELETE FROM enabled_vmbrs`,
			`INSERT INTO enabled_vmbrs (name, enabled) VALUES (?, 1)`,
			&summary.VMBRsCount},
		{"tags", coalesceSlice(src.Tags),
			`DELETE FROM tags`,
			`INSERT INTO tags (name) VALUES (?)`,
			&summary.TagsCount},
	}
	for i := range specs {
		n, err := migrateStringListTx(tx, &specs[i], changedBy)
		if err != nil {
			return err
		}
		*specs[i].countDst = n
	}
	return nil
}

func migrateStringListTx(tx *sql.Tx, spec *listSpec, changedBy string) (int, error) {
	if _, err := tx.Exec(spec.deleteSQL); err != nil {
		return 0, fmt.Errorf("delete %s: %w", spec.table, err)
	}
	count := 0
	for _, item := range spec.items {
		if item == "" {
			continue
		}
		if _, err := tx.Exec(spec.insertSQL, item); err != nil {
			return 0, fmt.Errorf("insert %s %q: %w", spec.table, item, err)
		}
		count++
	}
	newJSON, _ := json.Marshal(spec.items)
	if err := appendAudit(tx, spec.table, "list", "create", "", string(newJSON), changedBy); err != nil {
		return 0, err
	}
	return count, nil
}

func migrateCloudInitTemplatesTx(tx *sql.Tx, templates []JSONCloudInitTemplate, changedBy string, summary *MigrationSummary) error {
	for i := range templates {
		t := jsonToCloudInitTemplate(&templates[i])
		newJSON, _ := json.Marshal(t)
		inserted, err := execUpsertCloudInitTemplate(tx, t)
		if err != nil {
			return fmt.Errorf("upsert cloudinit_template %q: %w", t.ID, err)
		}
		action := "create"
		if !inserted {
			action = "update"
		}
		if err := appendAudit(tx, "cloudinit_templates", t.ID, action, "", string(newJSON), changedBy); err != nil {
			return err
		}
	}
	summary.CloudInitCount = len(templates)
	return nil
}

func migrateVMProfilesTx(tx *sql.Tx, profiles []JSONVMProfileConfig, changedBy string, summary *MigrationSummary) error {
	for i := range profiles {
		p, err := jsonToVMProfile(&profiles[i])
		if err != nil {
			return fmt.Errorf("encode vm_profile %q config: %w", profiles[i].ID, err)
		}
		newJSON, _ := json.Marshal(p)
		inserted, err := execUpsertVMProfile(tx, p)
		if err != nil {
			return fmt.Errorf("upsert vm_profile %q: %w", p.ID, err)
		}
		action := "create"
		if !inserted {
			action = "update"
		}
		if err := appendAudit(tx, "vm_profiles", p.ID, action, "", string(newJSON), changedBy); err != nil {
			return err
		}
	}
	summary.VMProfilesCount = len(profiles)
	return nil
}

func migrateSFTPConfigTx(tx *sql.Tx, src JSONSFTPConfig, changedBy string) error {
	cfg := jsonToSFTPConfig(src)
	newJSON, _ := json.Marshal(cfg)
	if err := execUpsertSFTPConfig(tx, cfg); err != nil {
		return fmt.Errorf("upsert sftp_config: %w", err)
	}
	return appendAudit(tx, "sftp_config", "1", "create", "", string(newJSON), changedBy)
}

func markBootstrapCompleteTx(tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO app_bootstrap (id, completed, completed_at, version)
		VALUES (1, 1, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(id) DO UPDATE SET
		    completed    = 1,
		    completed_at = CURRENT_TIMESTAMP,
		    version      = excluded.version
	`, "migration:settings.json")
	if err != nil {
		return fmt.Errorf("mark bootstrap complete: %w", err)
	}
	return nil
}

func appendMigrationAudit(tx *sql.Tx, summary *MigrationSummary, changedBy string) error {
	summJSON, _ := json.Marshal(summary)
	return appendAudit(tx, "migration", "settings.json", "create", "", string(summJSON), changedBy)
}

// ── Post-migration validation ─────────────────────────────────────────────────

// validateMigration counts rows in each table and compares with summary.
func validateMigration(impl *sqliteDB, summary *MigrationSummary) error {
	type check struct {
		query    string
		expected int
		table    string
	}
	checks := []check{
		{`SELECT COUNT(*) FROM enabled_nodes WHERE enabled = 1`, summary.NodesCount, "enabled_nodes"},
		{`SELECT COUNT(*) FROM enabled_storages WHERE enabled = 1`, summary.StoragesCount, "enabled_storages"},
		{`SELECT COUNT(*) FROM enabled_isos WHERE enabled = 1`, summary.ISOsCount, "enabled_isos"},
		{`SELECT COUNT(*) FROM enabled_vmbrs WHERE enabled = 1`, summary.VMBRsCount, "enabled_vmbrs"},
		{`SELECT COUNT(*) FROM tags`, summary.TagsCount, "tags"},
		{`SELECT COUNT(*) FROM cloudinit_templates`, summary.CloudInitCount, "cloudinit_templates"},
		{`SELECT COUNT(*) FROM vm_profiles`, summary.VMProfilesCount, "vm_profiles"},
	}
	for _, c := range checks {
		var count int
		if err := impl.db.QueryRow(c.query).Scan(&count); err != nil {
			return fmt.Errorf("validate %s: %w", c.table, err)
		}
		if count != c.expected {
			return fmt.Errorf("validate %s: expected %d rows, got %d", c.table, c.expected, count)
		}
	}
	return nil
}

// ── Converters ────────────────────────────────────────────────────────────────

// jsonToVMLimits maps JSONSettings flat fields to a VMLimits row.
// MaxVMs has no direct equivalent in settings.json; 0 means no global cap.
func jsonToVMLimits(src *JSONSettings) *VMLimits {
	const defaultMaxVMs = 0
	return &VMLimits{
		MaxVMs:          defaultMaxVMs,
		MaxVMPerUser:    src.MaxVMPerUser,
		MaxNetworkCards: src.MaxNetworkCards,
		MaxDiskPerVM:    src.MaxDiskPerVM,
		AllowCustomYAML: src.AllowCustomYAML,
		MaxSnapshots:    src.Limits.MaxSnapshots,
	}
}

func jsonToSFTPConfig(src JSONSFTPConfig) *SFTPConfig {
	return &SFTPConfig{
		Enabled:        src.Enabled,
		Host:           src.Host,
		Port:           src.Port,
		Username:       src.Username,
		PrivateKeyPath: src.PrivateKeyPath,
		RemotePath:     src.SnippetBaseDir,
	}
}

func jsonToCloudInitTemplate(src *JSONCloudInitTemplate) *CloudInitTemplate {
	return &CloudInitTemplate{
		ID:          src.ID,
		Name:        src.Name,
		Description: src.Description,
		Storage:     src.Storage,
		Filename:    src.Filename,
		YAMLContent: src.YAMLContent,
		Enabled:     src.Enabled,
	}
}

func jsonToVMProfile(src *JSONVMProfileConfig) (*VMProfile, error) {
	cfg := vmProfileConfigData{
		Sockets: src.Sockets,
		Cores:   src.Cores,
		RAMGB:   src.RAMGB,
		DiskGB:  src.DiskGB,
		DiskBus: src.DiskBus,
		Node:    src.Node,
		Storage: src.Storage,
		Icon:    src.Icon,
		Color:   src.Color,
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return &VMProfile{
		ID:          src.ID,
		Name:        src.Name,
		Description: src.Description,
		Config:      string(configJSON),
		Enabled:     src.Enabled,
	}, nil
}

// coalesceSlice returns s unchanged, or an empty (non-nil) slice for nil s.
func coalesceSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
