package apiv1

import (
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/cloudinit"
	"pvmss/database"
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
	isConfigured := cfg.Host != "" && cfg.Username != "" && cfg.PrivateKeyPath != ""

	keyExists := false
	if cfg.PrivateKeyPath != "" {
		if _, err := os.Stat(cfg.PrivateKeyPath); err == nil {
			keyExists = true
		}
	}

	if !cfg.Enabled {
		return &AdminSFTPStatusResponse{
			Enabled:      false,
			Host:         cfg.Host,
			Username:     cfg.Username,
			KeyExists:    keyExists,
			IsConfigured: isConfigured,
			StatusText:   "disabled",
			StatusType:   "warning",
		}
	}
	status := &AdminSFTPStatusResponse{
		Enabled:      true,
		Host:         cfg.Host,
		Username:     cfg.Username,
		KeyExists:    keyExists,
		IsConfigured: isConfigured,
	}
	if !keyExists {
		status.StatusText = "private-key-not-found"
		status.StatusType = "danger"
	} else if cfg.Host == "" {
		status.StatusText = "host-not-configured"
		status.StatusType = "danger"
	} else {
		status.StatusText = "configured"
		status.StatusType = "success"
	}
	return status
}

// ToggleSFTP handles POST /api/v1/admin/cloudinit/sftp/toggle.
func (h *AdminMutationsHandler) ToggleSFTP(w http.ResponseWriter, r *http.Request) {
	settings := h.state.GetSettings()
	cfg := settings.CloudInitSFTP
	newEnabled := !cfg.Enabled

	if h.state.HasDB() {
		dbCfg := &database.SFTPConfig{
			Enabled:        newEnabled,
			Host:           cfg.Host,
			Port:           cfg.Port,
			Username:       cfg.Username,
			PrivateKeyPath: cfg.PrivateKeyPath,
			RemotePath:     cfg.SnippetBaseDir,
		}
		if err := h.state.SetSFTPConfig(dbCfg, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.CloudInitSFTP = cfg
		newSettings.CloudInitSFTP.Enabled = newEnabled
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
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
