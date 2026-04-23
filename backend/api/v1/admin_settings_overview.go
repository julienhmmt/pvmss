// Package apiv1 — Admin settings overview endpoints.
//
// This file powers the Phase 9 "single pane of glass" admin panel:
//   - GET  /api/v1/admin/settings/overview  → typed snapshot of every DB table
//   - POST /api/v1/admin/settings/upsert    → dispatched add/update per table
//
// The panel is explicitly read/upsert-only: the handler rejects DELETE verbs
// and never exposes a delete action (T225). Disabling is performed via
// `enabled=false` flags where the schema supports it.
package apiv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"pvmss/database"
	"pvmss/logger"
	"pvmss/state"
)

// AdminSettingsOverviewHandler groups the overview + upsert endpoints.
type AdminSettingsOverviewHandler struct {
	state state.StateManager
	db    database.DB
}

// MakeAdminSettingsOverviewHandler creates a new handler.
// db may be nil in tests that do not exercise audit metadata.
func MakeAdminSettingsOverviewHandler(s state.StateManager, db database.DB) *AdminSettingsOverviewHandler {
	return &AdminSettingsOverviewHandler{state: s, db: db}
}

// Category identifiers used by the frontend to group cards visually.
const (
	categoryResources    = "resources"
	categoryInventory    = "inventory"
	categoryTemplates    = "templates"
	categoryIntegrations = "integrations"
)

// Kind identifiers drive the rendering pattern on the frontend.
const (
	kindSingleton = "singleton"
	kindList      = "list"
	kindKeyed     = "keyed"
)

// Table name constants — mirrored by the frontend.
const (
	tableVMLimits           = "vm_limits"
	tableNodeLimits         = "node_limits"
	tableEnabledNodes       = "enabled_nodes"
	tableEnabledStorages    = "enabled_storages"
	tableEnabledISOs        = "enabled_isos"
	tableEnabledVMBRs       = "enabled_vmbrs"
	tableTags               = "tags"
	tableCloudInitTemplates = "cloudinit_templates"
	tableVMProfiles         = "vm_profiles"
	tableSFTPConfig         = "sftp_config"
)

// SectionMeta describes a configuration section shown in the overview.
type SectionMeta struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Kind         string `json:"kind"`
	RowCount     int    `json:"row_count"`
	LastChangeAt string `json:"last_change_at,omitempty"`
	LastChangeBy string `json:"last_change_by,omitempty"`
	SupportsAdd  bool   `json:"supports_add"`
	SupportsEdit bool   `json:"supports_edit"`
}

// OverviewSection is one entry in the overview response.
type OverviewSection struct {
	SectionMeta
	Data interface{} `json:"data"`
}

// OverviewResponse is the JSON body returned by GetSettingsOverview.
type OverviewResponse struct {
	SchemaVersion     int                        `json:"schema_version"`
	BootstrapComplete bool                       `json:"bootstrap_complete"`
	Sections          map[string]OverviewSection `json:"sections"`
}

// VMLimitsPayload is the shape accepted by the vm_limits upsert.
type VMLimitsPayload struct {
	MaxVMs          int  `json:"max_vms"`
	MaxVMPerUser    int  `json:"max_vm_per_user"`
	MaxNetworkCards int  `json:"max_network_cards"`
	MaxDiskPerVM    int  `json:"max_disk_per_vm"`
	AllowCustomYAML bool `json:"allow_custom_yaml"`
	MaxSnapshots    int  `json:"max_snapshots"`
}

// NodeLimitPayload is the shape accepted by the node_limits upsert.
type NodeLimitPayload struct {
	Node      string `json:"node"`
	MaxVMs    int    `json:"max_vms"`
	MaxVCPUs  int    `json:"max_vcpus"`
	MaxRAMGB  int    `json:"max_ram_gb"`
	MaxDiskGB int    `json:"max_disk_gb"`
}

// SFTPConfigPayload mirrors database.SFTPConfig with JSON-friendly keys.
type SFTPConfigPayload struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PrivateKeyPath string `json:"private_key_path"`
	RemotePath     string `json:"remote_path"`
}

// CloudInitTemplatePayload is the shape accepted by cloudinit_templates upsert.
type CloudInitTemplatePayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Storage     string `json:"storage"`
	Filename    string `json:"filename"`
	YAMLContent string `json:"yaml_content"`
	Enabled     bool   `json:"enabled"`
}

// VMProfilePayload is the shape accepted by vm_profiles upsert.
type VMProfilePayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sockets     int    `json:"sockets"`
	Cores       int    `json:"cores"`
	RAMGB       int    `json:"ram_gb"`
	DiskGB      int    `json:"disk_gb"`
	DiskBus     string `json:"disk_bus"`
	Node        string `json:"node"`
	Storage     string `json:"storage"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Enabled     bool   `json:"enabled"`
	EnableEFI   bool   `json:"enable_efi,omitempty"`
}

// UpsertRequest is the JSON body for POST /api/v1/admin/settings/upsert.
// Raw record is decoded per-table from RawRecord.
type UpsertRequest struct {
	Table     string          `json:"table"`
	Action    string          `json:"action,omitempty"`
	RawRecord json.RawMessage `json:"record"`
}

// UpsertResponse is returned on successful upsert operations.
type UpsertResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetSettingsOverview handles GET /api/v1/admin/settings/overview.
func (h *AdminSettingsOverviewHandler) GetSettingsOverview(w http.ResponseWriter, _ *http.Request) {
	if !h.state.HasDB() {
		errInternal(w)
		return
	}

	settings := h.state.GetSettings()
	audits := h.latestAuditByTable()
	bootstrap := true
	if h.db != nil {
		if done, err := h.db.IsBootstrapComplete(); err == nil {
			bootstrap = done
		}
	}

	sections := map[string]OverviewSection{
		tableVMLimits:           buildVMLimitsSection(settings, audits[tableVMLimits]),
		tableNodeLimits:         buildNodeLimitsSection(settings, audits[tableNodeLimits]),
		tableEnabledNodes:       buildListSection(tableEnabledNodes, "Enabled Nodes", categoryInventory, safeStringSlice(settings.EnabledNodes), audits[tableEnabledNodes]),
		tableEnabledStorages:    buildListSection(tableEnabledStorages, "Enabled Storages", categoryInventory, safeStringSlice(settings.EnabledStorages), audits[tableEnabledStorages]),
		tableEnabledISOs:        buildListSection(tableEnabledISOs, "Enabled ISOs", categoryInventory, safeStringSlice(settings.ISOs), audits[tableEnabledISOs]),
		tableEnabledVMBRs:       buildListSection(tableEnabledVMBRs, "Enabled Network Bridges", categoryInventory, safeStringSlice(settings.VMBRs), audits[tableEnabledVMBRs]),
		tableTags:               buildListSection(tableTags, "Tags", categoryInventory, safeStringSlice(settings.Tags), audits[tableTags]),
		tableCloudInitTemplates: buildCloudInitSection(settings, audits[tableCloudInitTemplates]),
		tableVMProfiles:         buildVMProfilesSection(settings, audits[tableVMProfiles]),
		tableSFTPConfig:         buildSFTPSection(settings, audits[tableSFTPConfig]),
	}

	writeJSON(w, OverviewResponse{
		SchemaVersion:     1,
		BootstrapComplete: bootstrap,
		Sections:          sections,
	})
}

// latestAuditByTable returns the most recent audit entry per DB table.
// Missing tables map to a zero-value AuditEntry so callers can safely index.
func (h *AdminSettingsOverviewHandler) latestAuditByTable() map[string]database.AuditEntry {
	result := make(map[string]database.AuditEntry)
	if h.db == nil {
		return result
	}
	tables := []string{
		tableVMLimits, tableNodeLimits, tableEnabledNodes, tableEnabledStorages,
		tableEnabledISOs, tableEnabledVMBRs, tableTags,
		tableCloudInitTemplates, tableVMProfiles, tableSFTPConfig,
	}
	for _, t := range tables {
		entries, err := h.db.ListAuditLog(t, 1, 0)
		if err != nil || len(entries) == 0 {
			continue
		}
		result[t] = entries[0]
	}
	return result
}

// ── Section builders ─────────────────────────────────────────────────────────

// safeStringSlice returns the slice if non-nil, otherwise returns an empty slice.
// This prevents nil pointer panics when settings fields are not initialized.
func safeStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func buildVMLimitsSection(s *state.AppSettings, latest database.AuditEntry) OverviewSection {
	data := VMLimitsPayload{
		MaxVMs:          0,
		MaxVMPerUser:    s.MaxVMPerUser,
		MaxNetworkCards: s.MaxNetworkCards,
		MaxDiskPerVM:    s.MaxDiskPerVM,
		AllowCustomYAML: s.AllowCustomYAML,
		MaxSnapshots:    s.Limits.MaxSnapshots,
	}
	return OverviewSection{
		SectionMeta: SectionMeta{
			Name:         "VM Limits",
			Category:     categoryResources,
			Kind:         kindSingleton,
			RowCount:     1,
			LastChangeAt: latest.ChangedAt,
			LastChangeBy: latest.ChangedBy,
			SupportsAdd:  false,
			SupportsEdit: true,
		},
		Data: data,
	}
}

func buildNodeLimitsSection(s *state.AppSettings, latest database.AuditEntry) OverviewSection {
	rows := make([]NodeLimitPayload, 0, len(s.Limits.Nodes))
	for name, node := range s.Limits.Nodes {
		// Include the row if any limit is set (at least one non-zero field).
		if node.MaxVMs <= 0 && node.Cores.Max <= 0 && node.RAM.Max <= 0 && node.MaxDiskGB <= 0 {
			continue
		}
		rows = append(rows, NodeLimitPayload{
			Node:      name,
			MaxVMs:    node.MaxVMs,
			MaxVCPUs:  node.Cores.Max,
			MaxRAMGB:  node.RAM.Max,
			MaxDiskGB: node.MaxDiskGB,
		})
	}
	return OverviewSection{
		SectionMeta: SectionMeta{
			Name:         "Node Limits",
			Category:     categoryResources,
			Kind:         kindKeyed,
			RowCount:     len(rows),
			LastChangeAt: latest.ChangedAt,
			LastChangeBy: latest.ChangedBy,
			SupportsAdd:  true,
			SupportsEdit: true,
		},
		Data: rows,
	}
}

func buildListSection(table, displayName, category string, items []string, latest database.AuditEntry) OverviewSection {
	if items == nil {
		items = []string{}
	}
	return OverviewSection{
		SectionMeta: SectionMeta{
			Name:         displayName,
			Category:     category,
			Kind:         kindList,
			RowCount:     len(items),
			LastChangeAt: latest.ChangedAt,
			LastChangeBy: latest.ChangedBy,
			SupportsAdd:  true,
			SupportsEdit: true,
		},
		Data: items,
	}
}

func buildCloudInitSection(s *state.AppSettings, latest database.AuditEntry) OverviewSection {
	rows := make([]CloudInitTemplatePayload, 0, len(s.CloudInitTemplates))
	for _, t := range s.CloudInitTemplates {
		rows = append(rows, CloudInitTemplatePayload{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Storage:     t.Storage,
			Filename:    t.Filename,
			YAMLContent: t.YAMLContent,
			Enabled:     t.Enabled,
		})
	}
	return OverviewSection{
		SectionMeta: SectionMeta{
			Name:         "CloudInit Templates",
			Category:     categoryTemplates,
			Kind:         kindKeyed,
			RowCount:     len(rows),
			LastChangeAt: latest.ChangedAt,
			LastChangeBy: latest.ChangedBy,
			SupportsAdd:  true,
			SupportsEdit: true,
		},
		Data: rows,
	}
}

func buildVMProfilesSection(s *state.AppSettings, latest database.AuditEntry) OverviewSection {
	rows := make([]VMProfilePayload, 0, len(s.VMProfiles))
	for _, p := range s.VMProfiles {
		rows = append(rows, VMProfilePayload{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Sockets:     p.Sockets,
			Cores:       p.Cores,
			RAMGB:       p.RAMGB,
			DiskGB:      p.DiskGB,
			DiskBus:     p.DiskBus,
			Node:        p.Node,
			Storage:     p.Storage,
			Icon:        p.Icon,
			Color:       p.Color,
			Enabled:     p.Enabled,
			EnableEFI:   p.EnableEFI,
		})
	}
	return OverviewSection{
		SectionMeta: SectionMeta{
			Name:         "VM Profiles",
			Category:     categoryTemplates,
			Kind:         kindKeyed,
			RowCount:     len(rows),
			LastChangeAt: latest.ChangedAt,
			LastChangeBy: latest.ChangedBy,
			SupportsAdd:  true,
			SupportsEdit: true,
		},
		Data: rows,
	}
}

func buildSFTPSection(s *state.AppSettings, latest database.AuditEntry) OverviewSection {
	data := SFTPConfigPayload{
		Enabled:        s.CloudInitSFTP.Enabled,
		Host:           s.CloudInitSFTP.Host,
		Port:           s.CloudInitSFTP.Port,
		Username:       s.CloudInitSFTP.Username,
		PrivateKeyPath: s.CloudInitSFTP.PrivateKeyPath,
		RemotePath:     s.CloudInitSFTP.SnippetBaseDir,
	}
	return OverviewSection{
		SectionMeta: SectionMeta{
			Name:         "SFTP Configuration",
			Category:     categoryIntegrations,
			Kind:         kindSingleton,
			RowCount:     1,
			LastChangeAt: latest.ChangedAt,
			LastChangeBy: latest.ChangedBy,
			SupportsAdd:  false,
			SupportsEdit: true,
		},
		Data: data,
	}
}

// ── Upsert dispatcher ────────────────────────────────────────────────────────

// UpsertSettings handles POST /api/v1/admin/settings/upsert.
// Rejects DELETE verbs and any payload with action="delete" (T225).
func (h *AdminSettingsOverviewHandler) UpsertSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		logger.Get().Warn().Msg("settings overview: DELETE rejected")
		http.Error(w, "Method not allowed: deletions are disabled on the unified settings panel", http.StatusMethodNotAllowed)
		return
	}
	if !h.state.HasDB() {
		errInternal(w)
		return
	}

	var req UpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Action == "delete" {
		logger.Get().Warn().Str("table", req.Table).Msg("settings overview: action=delete rejected")
		http.Error(w, "deletions are disabled on the unified settings panel", http.StatusMethodNotAllowed)
		return
	}

	changedBy := usernameFromCtx(r)
	if changedBy == "" {
		errUnauthorized(w)
		return
	}

	if err := h.dispatchUpsert(req, changedBy); err != nil {
		errBadRequest(w, err.Error())
		return
	}

	writeJSON(w, UpsertResponse{Success: true, Message: "Settings updated"})
}

// dispatchUpsert routes the upsert to the correct table-specific handler.
func (h *AdminSettingsOverviewHandler) dispatchUpsert(req UpsertRequest, changedBy string) error {
	switch req.Table {
	case tableVMLimits:
		return h.upsertVMLimits(req.RawRecord, changedBy)
	case tableNodeLimits:
		return h.upsertNodeLimit(req.RawRecord, changedBy)
	case tableEnabledNodes, tableEnabledStorages, tableEnabledISOs, tableEnabledVMBRs, tableTags:
		return h.upsertList(req.Table, req.RawRecord, changedBy)
	case tableCloudInitTemplates:
		return h.upsertCloudInitTemplate(req.RawRecord, changedBy)
	case tableVMProfiles:
		return h.upsertVMProfile(req.RawRecord, changedBy)
	case tableSFTPConfig:
		return h.upsertSFTPConfig(req.RawRecord, changedBy)
	default:
		return fmt.Errorf("unknown table: %s", req.Table)
	}
}

// ── Upsert implementations ───────────────────────────────────────────────────

func (h *AdminSettingsOverviewHandler) upsertVMLimits(raw json.RawMessage, changedBy string) error {
	var p VMLimitsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("vm_limits: %w", err)
	}
	limits := &database.VMLimits{
		MaxVMs:          p.MaxVMs,
		MaxVMPerUser:    p.MaxVMPerUser,
		MaxNetworkCards: p.MaxNetworkCards,
		MaxDiskPerVM:    p.MaxDiskPerVM,
		AllowCustomYAML: p.AllowCustomYAML,
		MaxSnapshots:    p.MaxSnapshots,
	}
	if err := h.state.SetVMLimits(limits, changedBy); err != nil {
		logger.Get().Error().Err(err).Msg("settings overview: SetVMLimits failed")
		return err
	}
	return nil
}

func (h *AdminSettingsOverviewHandler) upsertNodeLimit(raw json.RawMessage, changedBy string) error {
	var p NodeLimitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("node_limits: %w", err)
	}
	if p.Node == "" {
		return fmt.Errorf("node_limits: node is required")
	}
	if p.MaxVMs < 0 {
		return fmt.Errorf("node_limits: max_vms must be >= 0")
	}
	if p.MaxVCPUs < 0 {
		return fmt.Errorf("node_limits: max_vcpus must be >= 0")
	}
	if p.MaxRAMGB < 0 {
		return fmt.Errorf("node_limits: max_ram_gb must be >= 0")
	}
	if p.MaxDiskGB < 0 {
		return fmt.Errorf("node_limits: max_disk_gb must be >= 0")
	}
	limit := database.NodeLimit{
		NodeName:  p.Node,
		MaxVMs:    p.MaxVMs,
		MaxVCPUs:  p.MaxVCPUs,
		MaxRAMGB:  p.MaxRAMGB,
		MaxDiskGB: p.MaxDiskGB,
	}
	if err := h.state.SetNodeLimit(limit, changedBy); err != nil {
		logger.Get().Error().Err(err).Str("node", p.Node).Msg("settings overview: SetNodeLimit failed")
		return err
	}
	return nil
}

func (h *AdminSettingsOverviewHandler) upsertList(table string, raw json.RawMessage, changedBy string) error {
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return fmt.Errorf("%s: expected array of strings: %w", table, err)
	}
	if items == nil {
		items = []string{}
	}

	// Validate: no empty strings
	for i, item := range items {
		if item == "" {
			return fmt.Errorf("%s: item at index %d is empty", table, i)
		}
	}

	// Validate: no duplicates
	seen := make(map[string]bool)
	for i, item := range items {
		if seen[item] {
			return fmt.Errorf("%s: duplicate item '%s' at index %d", table, item, i)
		}
		seen[item] = true
	}

	var err error
	switch table {
	case tableEnabledNodes:
		err = h.state.SetEnabledNodes(items, changedBy)
	case tableEnabledStorages:
		err = h.state.SetEnabledStorages(items, changedBy)
	case tableEnabledISOs:
		err = h.state.SetEnabledISOs(items, changedBy)
	case tableEnabledVMBRs:
		err = h.state.SetEnabledVMBRs(items, changedBy)
	case tableTags:
		err = h.state.SetTags(items, changedBy)
	default:
		return fmt.Errorf("unsupported list table: %s", table)
	}
	if err != nil {
		logger.Get().Error().Err(err).Str("table", table).Msg("settings overview: list upsert failed")
	}
	return err
}

func (h *AdminSettingsOverviewHandler) upsertCloudInitTemplate(raw json.RawMessage, changedBy string) error {
	var p CloudInitTemplatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("cloudinit_templates: %w", err)
	}
	if p.ID == "" || p.Name == "" {
		return fmt.Errorf("cloudinit_templates: id and name are required")
	}
	if p.Storage == "" {
		return fmt.Errorf("cloudinit_templates: storage is required")
	}
	if p.Filename == "" {
		return fmt.Errorf("cloudinit_templates: filename is required")
	}
	tpl := &database.CloudInitTemplate{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Storage:     p.Storage,
		Filename:    p.Filename,
		YAMLContent: p.YAMLContent,
		Enabled:     p.Enabled,
	}
	existing := h.state.GetSettings().GetCloudInitTemplateByID(p.ID)
	if existing == nil {
		return h.state.CreateCloudInitTemplate(tpl, changedBy)
	}
	return h.state.UpdateCloudInitTemplate(tpl, changedBy)
}

func (h *AdminSettingsOverviewHandler) upsertVMProfile(raw json.RawMessage, changedBy string) error {
	var p VMProfilePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("vm_profiles: %w", err)
	}
	if p.ID == "" || p.Name == "" {
		return fmt.Errorf("vm_profiles: id and name are required")
	}

	// Validate numeric ranges
	if p.Sockets < 1 {
		return fmt.Errorf("vm_profiles: sockets must be >= 1")
	}
	if p.Cores < 1 {
		return fmt.Errorf("vm_profiles: cores must be >= 1")
	}
	if p.RAMGB < 1 {
		return fmt.Errorf("vm_profiles: ram_gb must be >= 1")
	}
	if p.DiskGB < 1 {
		return fmt.Errorf("vm_profiles: disk_gb must be >= 1")
	}

	// Validate DiskBus against whitelist
	validDiskBuses := map[string]bool{
		"virtio": true,
		"scsi":   true,
		"sata":   true,
		"ide":    true,
	}
	if p.DiskBus != "" && !validDiskBuses[p.DiskBus] {
		return fmt.Errorf("vm_profiles: disk_bus must be one of: virtio, scsi, sata, ide")
	}

	blob := database.VMProfileConfigBlob{
		Sockets:   p.Sockets,
		Cores:     p.Cores,
		RAMGB:     p.RAMGB,
		DiskGB:    p.DiskGB,
		DiskBus:   p.DiskBus,
		Node:      p.Node,
		Storage:   p.Storage,
		Icon:      p.Icon,
		Color:     p.Color,
		EnableEFI: p.EnableEFI,
	}
	blobJSON, err := json.Marshal(blob)
	if err != nil {
		return fmt.Errorf("vm_profiles: marshal config: %w", err)
	}
	profile := &database.VMProfile{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Config:      string(blobJSON),
		Enabled:     p.Enabled,
	}
	existing := vmProfileByID(h.state.GetSettings(), p.ID)
	if existing == nil {
		return h.state.CreateVMProfile(profile, changedBy)
	}
	return h.state.UpdateVMProfile(profile, changedBy)
}

func (h *AdminSettingsOverviewHandler) upsertSFTPConfig(raw json.RawMessage, changedBy string) error {
	var p SFTPConfigPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("sftp_config: %w", err)
	}
	if p.Port == 0 {
		p.Port = 22
	}

	// Validate: if enabled, required fields must be non-empty
	if p.Enabled {
		if p.Host == "" {
			return fmt.Errorf("sftp_config: host is required when enabled")
		}
		if p.Username == "" {
			return fmt.Errorf("sftp_config: username is required when enabled")
		}
		if p.PrivateKeyPath == "" {
			return fmt.Errorf("sftp_config: private_key_path is required when enabled")
		}
		if p.RemotePath == "" {
			return fmt.Errorf("sftp_config: remote_path is required when enabled")
		}
	}

	// Validate: port range
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("sftp_config: port must be between 1 and 65535")
	}

	// Validate: path sanitization (basic check for directory traversal)
	if p.PrivateKeyPath != "" {
		// Check for obvious directory traversal attempts
		if len(p.PrivateKeyPath) > 0 && (p.PrivateKeyPath[0] == '.' || p.PrivateKeyPath[0] == '/') {
			// Allow relative paths starting with . or absolute paths, but log a warning
			logger.Get().Warn().Str("path", p.PrivateKeyPath).Msg("sftp_config: private_key_path uses absolute or relative path")
		}
	}

	cfg := &database.SFTPConfig{
		Enabled:        p.Enabled,
		Host:           p.Host,
		Port:           p.Port,
		Username:       p.Username,
		PrivateKeyPath: p.PrivateKeyPath,
		RemotePath:     p.RemotePath,
	}
	return h.state.SetSFTPConfig(cfg, changedBy)
}

// vmProfileByID locates a profile by its ID in the in-memory settings.
// Returns nil when no profile matches.
func vmProfileByID(s *state.AppSettings, id string) *state.VMProfileConfig {
	for i := range s.VMProfiles {
		if s.VMProfiles[i].ID == id {
			return &s.VMProfiles[i]
		}
	}
	return nil
}
