package apiv1

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/cloudinit"
	"pvmss/database"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// ListCloudInit handles GET /api/v1/admin/cloudinit.
func (h *AdminMutationsHandler) ListCloudInit(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	templates := make([]AdminCloudInitResponse, 0, len(settings.CloudInitTemplates))
	for _, t := range settings.CloudInitTemplates {
		templates = append(templates, AdminCloudInitResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Storage:     t.Storage,
			Filename:    t.Filename,
			YAMLContent: t.YAMLContent,
			Enabled:     t.Enabled,
		})
	}
	writeJSON(w, AdminCloudInitListResponse{
		Templates:  templates,
		SFTPStatus: buildSFTPStatus(settings),
	})
}

// buildSFTPStatus constructs SFTP status from settings without i18n (API returns machine-readable status).
func buildSFTPStatus(settings *state.AppSettings) *AdminSFTPStatusResponse {
	if settings == nil {
		return &AdminSFTPStatusResponse{
			Enabled:    false,
			StatusText: "settings-unavailable",
			StatusType: "danger",
		}
	}
	cfg := settings.CloudInitSFTP

	// A key is available either as pasted content (in-memory plaintext after
	// decryption) or as a readable key file on disk.
	keySet := cfg.PrivateKey != ""
	keyFileExists := false
	if cfg.PrivateKeyPath != "" {
		if _, err := os.Stat(cfg.PrivateKeyPath); err == nil {
			keyFileExists = true
		}
	}
	keyExists := keySet || keyFileExists

	fingerprint := ""
	if keySet {
		if fp, err := proxmox.SSHKeyFingerprint(cfg.PrivateKey); err == nil {
			fingerprint = fp
		}
	}

	isConfigured := cfg.Host != "" && cfg.Username != "" && keyExists

	base := AdminSFTPStatusResponse{
		Enabled:      cfg.Enabled,
		Host:         cfg.Host,
		Port:         cfg.Port,
		Username:     cfg.Username,
		RemotePath:   cfg.SnippetBaseDir,
		KeyExists:    keyExists,
		KeySet:       keySet,
		KeyPath:      cfg.PrivateKeyPath,
		HostKeyPath:  cfg.HostKeyPath,
		Fingerprint:  fingerprint,
		IsConfigured: isConfigured,
	}

	if !cfg.Enabled {
		base.StatusText = "disabled"
		base.StatusType = "warning"
		return &base
	}
	switch {
	case !keyExists:
		base.StatusText = "private-key-not-found"
		base.StatusType = "danger"
	case cfg.Host == "":
		base.StatusText = "host-not-configured"
		base.StatusType = "danger"
	default:
		base.StatusText = "configured"
		base.StatusType = "success"
	}
	return &base
}

// ToggleSFTP handles POST /api/v1/admin/cloudinit/sftp/toggle.
// It flips only the enabled flag, reading the raw persisted config so the stored
// (encrypted) private key and all other fields are preserved untouched.
func (h *AdminMutationsHandler) ToggleSFTP(w http.ResponseWriter, r *http.Request) {
	if !h.state.HasDB() {
		settings := h.state.GetSettings()
		newSettings := *settings
		newSettings.CloudInitSFTP = settings.CloudInitSFTP
		newSettings.CloudInitSFTP.Enabled = !settings.CloudInitSFTP.Enabled
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	dbCfg, err := h.state.GetSFTPConfig()
	if err != nil {
		writeAppError(w, err)
		return
	}
	dbCfg.Enabled = !dbCfg.Enabled
	if err := h.state.SetSFTPConfig(dbCfg, usernameFromCtx(r)); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateSFTPConfig handles PUT /api/v1/admin/cloudinit/sftp.
// Persists host/port/username/remote-path and, when a non-empty private_key is
// supplied, validates and encrypts it before storing. A blank private_key keeps
// the currently stored key. The enabled flag is preserved (use the toggle).
func (h *AdminMutationsHandler) UpdateSFTPConfig(w http.ResponseWriter, r *http.Request) {
	if !h.state.HasDB() {
		writeError(w, http.StatusBadRequest, "no_database", "SFTP configuration requires a database")
		return
	}
	var req AdminSFTPConfigRequest
	if !decodeBody(w, r, &req) {
		return
	}

	existing, err := h.state.GetSFTPConfig()
	if err != nil {
		writeAppError(w, err)
		return
	}

	port := req.Port
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		errBadRequest(w, "port must be between 1 and 65535")
		return
	}

	dbCfg := &database.SFTPConfig{
		Enabled:        existing.Enabled,
		Host:           strings.TrimSpace(req.Host),
		Port:           port,
		Username:       strings.TrimSpace(req.Username),
		PrivateKeyPath: strings.TrimSpace(req.PrivateKeyPath),
		RemotePath:     strings.TrimSpace(req.RemotePath),
		HostKeyPath:    strings.TrimSpace(req.HostKeyPath),
		PrivateKey:     existing.PrivateKey, // preserve stored (encrypted) key by default
	}

	if key := strings.TrimSpace(req.PrivateKey); key != "" {
		// Validate the key parses before storing so we never persist garbage.
		if _, err := proxmox.SSHKeyFingerprint(key); err != nil {
			errBadRequest(w, "invalid private key: "+err.Error())
			return
		}
		secret := h.state.GetEnvConfig().SessionSecret
		enc, err := security.EncryptSecret(key, secret)
		if err != nil {
			writeAppError(w, err)
			return
		}
		dbCfg.PrivateKey = enc
	}

	if err := h.state.SetSFTPConfig(dbCfg, usernameFromCtx(r)); err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, buildSFTPStatus(h.state.GetSettings()))
}

// TestSFTPConnection handles POST /api/v1/admin/cloudinit/sftp/test.
// Dials the configured SFTP server and writes+removes a probe file to verify
// connectivity, authentication, and write permission.
func (h *AdminMutationsHandler) TestSFTPConnection(w http.ResponseWriter, r *http.Request) {
	cfg := h.state.GetSettings().CloudInitSFTP
	if cfg.Host == "" || cfg.Username == "" {
		errBadRequest(w, "host and username must be set before testing")
		return
	}
	if cfg.PrivateKey == "" && cfg.PrivateKeyPath == "" {
		errBadRequest(w, "a private key must be set before testing")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Allow testing before the feature is toggled on.
	cfg.Enabled = true
	if err := proxmox.TestSFTPConnection(ctx, cfg); err != nil {
		writeJSON(w, map[string]any{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"success": true, "message": "SFTP connection OK"})
}

// generateCloudInitID generates a safe ID from a template name (lowercase, alphanumeric, hyphens).
func generateCloudInitID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	id = cloudInitIDUnsafeRegex.ReplaceAllString(id, "")
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	id = strings.Trim(id, "-")
	if len(id) < 2 {
		id = "template-" + id
	}
	return id
}

// CreateCloudInit handles POST /api/v1/admin/cloudinit.
func (h *AdminMutationsHandler) CreateCloudInit(w http.ResponseWriter, r *http.Request) {
	var req CreateCloudInitRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" || req.YAMLContent == "" {
		errBadRequest(w, "name and yaml_content are required")
		return
	}
	if err := cloudinit.ValidateCloudInitYAMLStrict(req.YAMLContent); err != nil {
		errBadRequest(w, "invalid YAML: "+err.Error())
		return
	}

	id := generateCloudInitID(req.Name)
	filename := state.CloudInitTemplatePrefix + id + ".yml"

	settings := h.state.GetSettings()
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			errBadRequest(w, "a template with this name already exists")
			return
		}
	}

	template := state.CloudInitTemplate{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Storage:     req.Storage,
		Filename:    filename,
		YAMLContent: req.YAMLContent,
		Enabled:     true,
	}

	if h.state.HasDB() {
		dbTemplate := &database.CloudInitTemplate{
			ID: template.ID, Name: template.Name, Description: template.Description,
			Storage: template.Storage, Filename: template.Filename,
			YAMLContent: template.YAMLContent, Enabled: template.Enabled,
		}
		if err := h.state.CreateCloudInitTemplate(dbTemplate, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates), len(settings.CloudInitTemplates)+1)
		copy(newTemplates, settings.CloudInitTemplates)
		newTemplates = append(newTemplates, template)
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, AdminCloudInitResponse{
		ID: id, Name: req.Name, Description: req.Description,
		Storage: req.Storage, Filename: filename, YAMLContent: req.YAMLContent, Enabled: true,
	})
}

// UpdateCloudInit handles PUT /api/v1/admin/cloudinit/:id.
func (h *AdminMutationsHandler) UpdateCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}
	var req UpdateCloudInitRequest
	if !decodeBody(w, r, &req) {
		return
	}

	if req.YAMLContent != "" {
		if err := cloudinit.ValidateCloudInitYAMLStrict(req.YAMLContent); err != nil {
			errBadRequest(w, "invalid YAML: "+err.Error())
			return
		}
	}

	settings := h.state.GetSettings()
	found := false
	var existing state.CloudInitTemplate
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			existing = t
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.Storage = req.Storage
	if req.YAMLContent != "" {
		existing.YAMLContent = req.YAMLContent
	}
	if h.state.HasDB() {
		dbTemplate := &database.CloudInitTemplate{
			ID: existing.ID, Name: existing.Name, Description: existing.Description,
			Storage: existing.Storage, Filename: existing.Filename,
			YAMLContent: existing.YAMLContent, Enabled: existing.Enabled,
		}
		if err := h.state.UpdateCloudInitTemplate(dbTemplate, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates))
		copy(newTemplates, settings.CloudInitTemplates)
		for i, t := range newTemplates {
			if t.ID == id {
				newTemplates[i] = existing
				break
			}
		}
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteCloudInit handles DELETE /api/v1/admin/cloudinit/:id.
func (h *AdminMutationsHandler) DeleteCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}

	settings := h.state.GetSettings()
	found := false
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	if h.state.HasDB() {
		if err := h.state.DeleteCloudInitTemplate(id, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, 0, len(settings.CloudInitTemplates))
		for _, t := range settings.CloudInitTemplates {
			if t.ID == id {
				continue
			}
			newTemplates = append(newTemplates, t)
		}
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleCloudInit handles POST /api/v1/admin/cloudinit/:id/toggle.
func (h *AdminMutationsHandler) ToggleCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}

	settings := h.state.GetSettings()
	found := false
	var toggled state.CloudInitTemplate
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			t.Enabled = !t.Enabled
			toggled = t
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	if h.state.HasDB() {
		dbTemplate := &database.CloudInitTemplate{
			ID: toggled.ID, Name: toggled.Name, Description: toggled.Description,
			Storage: toggled.Storage, Filename: toggled.Filename,
			YAMLContent: toggled.YAMLContent, Enabled: toggled.Enabled,
		}
		if err := h.state.UpdateCloudInitTemplate(dbTemplate, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates))
		copy(newTemplates, settings.CloudInitTemplates)
		for i, t := range newTemplates {
			if t.ID == id {
				newTemplates[i].Enabled = toggled.Enabled
				break
			}
		}
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
