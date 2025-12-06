package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/cloudinit"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// CloudInitHandler handles cloud-init template management.
type CloudInitHandler struct {
	stateManager state.StateManager
}

// NewCloudInitHandler creates a new CloudInitHandler.
func NewCloudInitHandler(stateManager state.StateManager) *CloudInitHandler {
	return &CloudInitHandler{stateManager: stateManager}
}

// RegisterRoutes registers cloud-init admin routes.
func (h *CloudInitHandler) RegisterRoutes(router *httprouter.Router) {
	log := CreateHandlerLogger("CloudInitHandler", nil)
	log.Debug().Msg("Registering cloud-init admin routes")

	// Admin cloud-init page
	router.GET("/admin/cloudinit", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.CloudInitPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Create template
	router.POST("/admin/cloudinit/create", SecureFormHandler("CloudInitCreate",
		func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			h.CreateTemplateHandler(w, r, ps)
		}))

	// Edit template
	router.POST("/admin/cloudinit/edit", SecureFormHandler("CloudInitEdit",
		func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			h.EditTemplateHandler(w, r, ps)
		}))

	// Delete template
	router.POST("/admin/cloudinit/delete", SecureFormHandler("CloudInitDelete",
		func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			h.DeleteTemplateHandler(w, r, ps)
		}))

	// Toggle template enabled state
	router.POST("/admin/cloudinit/toggle", SecureFormHandler("CloudInitToggle",
		func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			h.ToggleTemplateHandler(w, r, ps)
		}))

	// Get single template with content (admin view)
	router.GET("/admin/cloudinit/template/:id", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetTemplateHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Public API for authenticated users to view template content (used on VM create page)
	router.GET("/api/cloudinit/template/:id", RequireAuthHandle(h.GetTemplateHandler))

	log.Info().Msg("Cloud-init admin routes registered")
}

// CloudInitPageHandler renders the cloud-init admin page.
func (h *CloudInitHandler) CloudInitPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CloudInitPageHandler", r)

	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	settings := h.stateManager.GetSettings()

	// Check if user has a valid Proxmox session (can create/edit/delete templates)
	// Local admins (authenticated via password hash) don't have a Proxmox session
	hasProxmoxSession := h.hasProxmoxSession(r)
	log.Debug().
		Bool("proxmox_connected", proxmoxConnected).
		Bool("has_proxmox_session", hasProxmoxSession).
		Msg("Checking user's Proxmox session status for cloud-init page")

	var snippetStorages []proxmox.Storage
	var errMsg string

	if proxmoxConnected {
		log.Debug().Msg("Proxmox is connected, fetching snippet storages")
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create resty client")
			errMsg = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxConnection")
		} else {
			storages, err := proxmox.GetSnippetsStoragesResty(r.Context(), restyClient)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get snippets storages")
				errMsg = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.FetchStorages")
			} else {
				log.Info().Int("count", len(storages)).Msg("Successfully retrieved snippet storages")
				for _, storage := range storages {
					log.Debug().Str("storage", storage.Storage).Str("type", storage.Type).Msg("Available snippet storage")
				}
				snippetStorages = storages
			}
		}
	} else {
		log.Warn().Msg("Proxmox is not connected, no snippet storages available")
	}

	// Check SFTP configuration status
	sftpStatus := h.getSFTPStatus(settings)

	// Build template data using functional options pattern
	opts := []TemplateOption{
		WithAdminActive("cloudinit"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Admin.CloudInit.Title"),
		WithData("Templates", settings.CloudInitTemplates),
		WithData("SnippetStorages", snippetStorages),
		WithData("AllowCustomYAML", settings.AllowCustomYAML),
		WithData("ProxmoxConnected", proxmoxConnected),
		WithData("HasProxmoxSession", hasProxmoxSession),
		WithData("SFTPStatus", sftpStatus),
	}

	if errMsg != "" {
		opts = append(opts, WithError(errMsg))
	}

	// Check for success message from redirect
	if r.URL.Query().Get("success") != "" {
		action := r.URL.Query().Get("action")
		name := r.URL.Query().Get("name")
		localizer := i18n.GetLocalizerFromRequest(r)

		var successMsg string
		switch action {
		case "create":
			successMsg = i18n.Localize(localizer, "Admin.CloudInit.Success.Create")
		case "edit":
			successMsg = i18n.Localize(localizer, "Admin.CloudInit.Success.Edit")
		case "delete":
			successMsg = i18n.Localize(localizer, "Admin.CloudInit.Success.Delete")
		case "enable":
			successMsg = i18n.Localize(localizer, "Admin.CloudInit.Success.Enable")
		case "disable":
			successMsg = i18n.Localize(localizer, "Admin.CloudInit.Success.Disable")
		default:
			successMsg = i18n.Localize(localizer, "Admin.CloudInit.Success.Default")
		}
		if name != "" && strings.Contains(successMsg, "%s") {
			successMsg = strings.Replace(successMsg, "%s", name, 1)
		}
		opts = append(opts, WithSuccess(successMsg))
	}

	// Check for warning message from redirect (e.g., Proxmox upload failed but local storage succeeded)
	if warning := r.URL.Query().Get("warning"); warning != "" {
		name := r.URL.Query().Get("name")
		localizer := i18n.GetLocalizerFromRequest(r)

		var warningMsg string
		switch warning {
		case "ProxmoxUploadFailed":
			warningMsg = i18n.Localize(localizer, "Admin.CloudInit.Warning.ProxmoxUploadFailed")
		default:
			warningMsg = i18n.Localize(localizer, "Admin.CloudInit.Warning.Default")
		}
		if name != "" && strings.Contains(warningMsg, "%s") {
			warningMsg = strings.Replace(warningMsg, "%s", name, 1)
		}
		opts = append(opts, WithWarning(warningMsg))
	}

	data := NewTemplateDataWithOptions("", opts...).ToMap()
	renderTemplateInternal(w, r, "admin_cloudinit", data)
}

// CreateTemplateHandler handles creating a new cloud-init template.
func (h *CloudInitHandler) CreateTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Check if user has a valid Proxmox session
	// Local admins (authenticated via password hash) cannot create cloud-init templates
	// because they don't have Proxmox credentials to upload files
	if !h.hasProxmoxSession(r) {
		log.Warn().Msg("User attempted to create cloud-init template without valid Proxmox session")
		h.redirectWithError(w, r, "Admin.CloudInit.Error.NoProxmoxSession")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	storage := strings.TrimSpace(r.FormValue("storage"))
	yamlContent := r.FormValue("yaml_content")

	// Validate required fields (storage is optional when templates are stored locally)
	if name == "" || yamlContent == "" {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.RequiredFields")
		return
	}

	// Validate YAML with strict cloud-init rules (requires #cloud-config header)
	if err := cloudinit.ValidateCloudInitYAMLStrict(yamlContent); err != nil {
		log.Warn().Err(err).Str("name", name).Msg("Invalid YAML content")
		h.redirectWithError(w, r, "Admin.CloudInit.Error.InvalidYAML")
		return
	}

	// Generate safe ID from name
	id := generateSafeID(name)
	filename := state.CloudInitTemplatePrefix + id + ".yml"

	// Check if template already exists
	settings := h.stateManager.GetSettings()
	if settings.GetCloudInitTemplateByID(id) != nil {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.AlreadyExists")
		return
	}

	// Add template to settings with YAML content (stored locally only)
	template := state.CloudInitTemplate{
		ID:          id,
		Name:        name,
		Description: description,
		Storage:     storage,
		Filename:    filename,
		YAMLContent: yamlContent,
		Enabled:     true, // Enabled by default
	}
	settings.AddOrUpdateCloudInitTemplate(template)

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Str("name", name).Msg("Failed to save settings")
		h.redirectWithError(w, r, "Error.InternalServer")
		return
	}

	// Audit log
	username := h.getUsername(r)
	logger.AdminEvent("cloudinit_template_create", username).
		Str("template_id", id).
		Str("template_name", name).
		Str("storage", storage).
		Str("client_ip", r.RemoteAddr).
		Msg("Cloud-init template created")

	// Redirect with success message
	http.Redirect(w, r, "/admin/cloudinit?success=1&action=create&name="+url.QueryEscape(name), http.StatusSeeOther)
}

// EditTemplateHandler handles editing an existing cloud-init template.
func (h *CloudInitHandler) EditTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("EditTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Check if user has a valid Proxmox session for uploading updated content
	if !h.hasProxmoxSession(r) {
		log.Warn().Msg("User attempted to edit cloud-init template without valid Proxmox session")
		h.redirectWithError(w, r, "Admin.CloudInit.Error.NoProxmoxSession")
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	yamlContent := r.FormValue("yaml_content")

	if id == "" || name == "" {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.RequiredFields")
		return
	}

	settings := h.stateManager.GetSettings()
	template := settings.GetCloudInitTemplateByID(id)
	if template == nil {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.NotFound")
		return
	}

	// Validate YAML if provided (use strict validation requiring #cloud-config header)
	if yamlContent != "" {
		if err := cloudinit.ValidateCloudInitYAMLStrict(yamlContent); err != nil {
			log.Warn().Err(err).Str("id", id).Msg("Invalid YAML content")
			h.redirectWithError(w, r, "Admin.CloudInit.Error.InvalidYAML")
			return
		}
	}

	// Update template metadata and YAML content (stored locally only)
	template.Name = name
	template.Description = description
	if yamlContent != "" {
		template.YAMLContent = yamlContent
	}
	settings.AddOrUpdateCloudInitTemplate(*template)

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to save settings")
		h.redirectWithError(w, r, "Error.InternalServer")
		return
	}

	// Audit log
	username := h.getUsername(r)
	logger.AdminEvent("cloudinit_template_edit", username).
		Str("template_id", id).
		Str("template_name", name).
		Str("client_ip", r.RemoteAddr).
		Msg("Cloud-init template edited")

	http.Redirect(w, r, "/admin/cloudinit?success=1&action=edit&name="+url.QueryEscape(name), http.StatusSeeOther)
}

// DeleteTemplateHandler handles deleting a cloud-init template.
func (h *CloudInitHandler) DeleteTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Check if user has a valid Proxmox session for deleting files
	if !h.hasProxmoxSession(r) {
		log.Warn().Msg("User attempted to delete cloud-init template without valid Proxmox session")
		h.redirectWithError(w, r, "Admin.CloudInit.Error.NoProxmoxSession")
		return
	}

	// Get ID from form data (not URL params)
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.RequiredFields")
		return
	}

	// Get settings to find template
	settings := h.stateManager.GetSettings()

	// Find template by ID
	var templateToDelete *state.CloudInitTemplate
	for i := range settings.CloudInitTemplates {
		if settings.CloudInitTemplates[i].ID == id {
			templateToDelete = &settings.CloudInitTemplates[i]
			break
		}
	}

	if templateToDelete == nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Remove from settings
	newTemplates := make([]state.CloudInitTemplate, 0, len(settings.CloudInitTemplates))
	for _, tmpl := range settings.CloudInitTemplates {
		if tmpl.ID != id {
			newTemplates = append(newTemplates, tmpl)
		}
	}
	settings.CloudInitTemplates = newTemplates

	// Save settings
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Redirect with success message
	http.Redirect(w, r, fmt.Sprintf("/admin/cloudinit?success=1&action=delete&name=%s", url.QueryEscape(templateToDelete.Name)), http.StatusSeeOther)
}

// GetTemplateHandler returns a single template with its YAML content
func (h *CloudInitHandler) GetTemplateHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("GetTemplateHandler", r)

	// Prefer explicit route params, but fall back to params stored in context
	id := ps.ByName("id")
	if id == "" {
		if ctxParams, ok := r.Context().Value(ParamsKey).(httprouter.Params); ok {
			id = ctxParams.ByName("id")
		}
	}
	if id == "" {
		http.Error(w, "Template ID is required", http.StatusBadRequest)
		return
	}

	// Get settings to find template
	settings := h.stateManager.GetSettings()

	// Find template by ID
	var template *state.CloudInitTemplate
	for i := range settings.CloudInitTemplates {
		if settings.CloudInitTemplates[i].ID == id {
			template = &settings.CloudInitTemplates[i]
			break
		}
	}

	if template == nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Prepare response using local YAML content only
	content := template.YAMLContent
	source := "none"
	if content != "" {
		source = "local"
	}

	response := map[string]interface{}{
		"id":          template.ID,
		"name":        template.Name,
		"description": template.Description,
		"storage":     template.Storage,
		"content":     content,
		"source":      source,
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// ToggleTemplateHandler handles enabling/disabling a cloud-init template.
func (h *CloudInitHandler) ToggleTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ToggleTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	action := strings.TrimSpace(r.FormValue("action")) // "enable" or "disable"

	if id == "" || (action != "enable" && action != "disable") {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.RequiredFields")
		return
	}

	settings := h.stateManager.GetSettings()
	template := settings.GetCloudInitTemplateByID(id)
	if template == nil {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.NotFound")
		return
	}

	enabled := action == "enable"
	if !settings.SetCloudInitTemplateEnabled(id, enabled) {
		h.redirectWithError(w, r, "Error.InternalServer")
		return
	}

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to save settings")
		h.redirectWithError(w, r, "Error.InternalServer")
		return
	}

	// Audit log
	username := h.getUsername(r)
	logger.AdminEvent("cloudinit_template_toggle", username).
		Str("template_id", id).
		Str("template_name", template.Name).
		Str("action", action).
		Str("client_ip", r.RemoteAddr).
		Msg("Cloud-init template toggled")

	http.Redirect(w, r, "/admin/cloudinit?success=1&action="+action+"&name="+url.QueryEscape(template.Name), http.StatusSeeOther)
}

// Helper functions

func (h *CloudInitHandler) redirectWithError(w http.ResponseWriter, r *http.Request, messageKey string) {
	ctx := NewHandlerContext(w, r, "CloudInitHandler")
	ctx.RedirectWithError("/admin/cloudinit", messageKey)
}

func (h *CloudInitHandler) getUsername(r *http.Request) string {
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			return user
		}
	}
	return ""
}

// hasProxmoxSession checks if the current user has a valid Proxmox session.
// Local admins (who authenticate via password hash) do NOT have a Proxmox session
// and therefore cannot create/edit/delete cloud-init templates on Proxmox storage.
func (h *CloudInitHandler) hasProxmoxSession(r *http.Request) bool {
	// Check if Proxmox is connected at all
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	if !proxmoxConnected {
		return false
	}

	// Check if user has a valid Proxmox ticket in their session
	return IsProxmoxTicketValid(r)
}

// SFTPStatus represents the status of SFTP configuration for cloud-init uploads.
type SFTPStatus struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Username   string `json:"username"`
	KeyExists  bool   `json:"key_exists"`
	StatusText string `json:"status_text"`
	StatusType string `json:"status_type"` // "success", "warning", "danger"
}

// getSFTPStatus checks the SFTP configuration and returns status information.
func (h *CloudInitHandler) getSFTPStatus(settings *state.AppSettings) SFTPStatus {
	if settings == nil {
		return SFTPStatus{
			Enabled:    false,
			StatusText: "Settings not available",
			StatusType: "danger",
		}
	}

	cfg := settings.CloudInitSFTP
	if !cfg.Enabled {
		return SFTPStatus{
			Enabled:    false,
			StatusText: "SFTP upload disabled",
			StatusType: "warning",
		}
	}

	// Check if private key file exists
	keyExists := false
	if cfg.PrivateKeyPath != "" {
		if _, err := os.Stat(cfg.PrivateKeyPath); err == nil {
			keyExists = true
		}
	}

	status := SFTPStatus{
		Enabled:   true,
		Host:      cfg.Host,
		Username:  cfg.Username,
		KeyExists: keyExists,
	}

	if !keyExists {
		status.StatusText = "Private key file not found"
		status.StatusType = "danger"
	} else if cfg.Host == "" {
		status.StatusText = "SFTP host not configured"
		status.StatusType = "danger"
	} else {
		status.StatusText = "SFTP configured and ready"
		status.StatusType = "success"
	}

	return status
}

// generateSafeID generates a safe ID from a name (lowercase, alphanumeric, hyphens).
func generateSafeID(name string) string {
	// Convert to lowercase
	id := strings.ToLower(name)
	// Replace spaces and underscores with hyphens
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	// Remove any non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	id = reg.ReplaceAllString(id, "")
	// Remove consecutive hyphens
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	// Trim leading/trailing hyphens
	id = strings.Trim(id, "-")
	// Ensure minimum length
	if len(id) < 2 {
		id = "template-" + id
	}
	return id
}
