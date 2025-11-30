package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

	// Get single template with content
	router.GET("/admin/cloudinit/template/:id", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetTemplateHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	log.Info().Msg("Cloud-init admin routes registered")
}

// CloudInitPageHandler renders the cloud-init admin page.
func (h *CloudInitHandler) CloudInitPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CloudInitPageHandler", r)

	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	settings := h.stateManager.GetSettings()

	var snippetStorages []proxmox.Storage
	var errMsg string

	if proxmoxConnected {
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
				snippetStorages = storages
			}
		}
	}

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

	data := NewTemplateDataWithOptions("", opts...).ToMap()
	renderTemplateInternal(w, r, "admin_cloudinit", data)
}

// CreateTemplateHandler handles creating a new cloud-init template.
func (h *CloudInitHandler) CreateTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	storage := strings.TrimSpace(r.FormValue("storage"))
	yamlContent := r.FormValue("yaml_content")

	// Validate required fields
	if name == "" || storage == "" || yamlContent == "" {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.RequiredFields")
		return
	}

	// Validate YAML
	if err := cloudinit.ValidateCloudInitYAML(yamlContent); err != nil {
		log.Warn().Err(err).Str("name", name).Msg("Invalid YAML content")
		h.redirectWithError(w, r, "Admin.CloudInit.Error.InvalidYAML")
		return
	}

	// Generate safe ID from name
	id := generateSafeID(name)
	filename := state.CloudInitTemplatePrefix + id + ".yaml"

	// Check if template already exists
	settings := h.stateManager.GetSettings()
	if settings.GetCloudInitTemplateByID(id) != nil {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.AlreadyExists")
		return
	}

	// Upload snippet file to Proxmox storage
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Resty client")
		h.redirectWithError(w, r, "Error.ProxmoxUploadFailed")
		return
	}

	// Get the first available online node
	var node string
	if nodes, err := proxmox.GetOnlineNodeNamesResty(r.Context(), restyClient); err == nil && len(nodes) > 0 {
		node = nodes[0] // Use first online node
	} else {
		log.Warn().Err(err).Msg("Failed to get online nodes, using localhost fallback")
		node = "localhost"
	}

	// Upload the YAML content to Proxmox storage
	if err := proxmox.UploadSnippetFileResty(r.Context(), restyClient, node, storage, filename, yamlContent); err != nil {
		log.Error().Err(err).Str("storage", storage).Str("filename", filename).Msg("Failed to upload snippet file to Proxmox")
		h.redirectWithError(w, r, "Error.ProxmoxUploadFailed")
		return
	}
	log.Info().Str("storage", storage).Str("filename", filename).Str("node", node).Msg("Successfully uploaded snippet file to Proxmox")

	// Add template to settings
	template := state.CloudInitTemplate{
		ID:          id,
		Name:        name,
		Description: description,
		Storage:     storage,
		Filename:    filename,
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

	http.Redirect(w, r, "/admin/cloudinit?success=1&action=create&name="+url.QueryEscape(name), http.StatusSeeOther)
}

// EditTemplateHandler handles editing an existing cloud-init template.
func (h *CloudInitHandler) EditTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("EditTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
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

	// Validate YAML if provided
	if yamlContent != "" {
		if err := cloudinit.ValidateCloudInitYAML(yamlContent); err != nil {
			log.Warn().Err(err).Str("id", id).Msg("Invalid YAML content")
			h.redirectWithError(w, r, "Admin.CloudInit.Error.InvalidYAML")
			return
		}
	}

	// Upload updated YAML content to Proxmox storage if provided
	if yamlContent != "" {
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create Resty client")
			h.redirectWithError(w, r, "Error.ProxmoxUploadFailed")
			return
		}

		// Get the first available online node
		var node string
		if nodes, err := proxmox.GetOnlineNodeNamesResty(r.Context(), restyClient); err == nil && len(nodes) > 0 {
			node = nodes[0] // Use first online node
		} else {
			log.Warn().Err(err).Msg("Failed to get online nodes, using localhost fallback")
			node = "localhost"
		}

		// Upload the updated YAML content to Proxmox storage
		if err := proxmox.UploadSnippetFileResty(r.Context(), restyClient, node, template.Storage, template.Filename, yamlContent); err != nil {
			log.Error().Err(err).Str("storage", template.Storage).Str("filename", template.Filename).Msg("Failed to upload updated snippet file to Proxmox")
			h.redirectWithError(w, r, "Error.ProxmoxUploadFailed")
			return
		}
		log.Info().Str("storage", template.Storage).Str("filename", template.Filename).Str("node", node).Msg("Successfully uploaded updated snippet file to Proxmox")
	}

	// Update template metadata
	template.Name = name
	template.Description = description
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
func (h *CloudInitHandler) DeleteTemplateHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("DeleteTemplateHandler", r)

	id := ps.ByName("id")
	if id == "" {
		http.Error(w, "Template ID is required", http.StatusBadRequest)
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

	// Delete from Proxmox if connected
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	if proxmoxConnected {
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create resty client")
			http.Error(w, "Failed to connect to Proxmox", http.StatusInternalServerError)
			return
		}

		// Get node information to find an active node
		snapshot := h.stateManager.GetProxmoxSnapshot()
		if snapshot == nil || len(snapshot.NodeDetails) == 0 {
			http.Error(w, "No node information available", http.StatusInternalServerError)
			return
		}
		activeNode := snapshot.NodeDetails[0].Node
		if activeNode == "" {
			http.Error(w, "No active node available", http.StatusInternalServerError)
			return
		}

		// Delete file from Proxmox snippets storage
		volid := fmt.Sprintf("%s:snippets/%s", templateToDelete.Storage, templateToDelete.Filename)
		err = proxmox.DeleteSnippetFileResty(r.Context(), restyClient, activeNode, templateToDelete.Storage, volid)
		if err != nil {
			log.Error().Err(err).
				Str("storage", templateToDelete.Storage).
				Str("filename", templateToDelete.Filename).
				Msg("Failed to delete template file from Proxmox")
			http.Error(w, "Failed to delete template file from Proxmox", http.StatusInternalServerError)
			return
		}
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

	id := ps.ByName("id")
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

	// Prepare response
	response := map[string]interface{}{
		"id":          template.ID,
		"name":        template.Name,
		"description": template.Description,
		"storage":     template.Storage,
		"content":     "", // Will be populated if Proxmox is connected
	}

	// Fetch YAML content from Proxmox if connected
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	if proxmoxConnected {
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create resty client")
			// Continue without content - template metadata is still useful
		} else {
			// Get node information to find an active node
			snapshot := h.stateManager.GetProxmoxSnapshot()
			if snapshot != nil && len(snapshot.NodeDetails) > 0 {
				// Use first available node
				activeNode := snapshot.NodeDetails[0].Node
				// Download file content from Proxmox snippets storage
				volid := fmt.Sprintf("%s:snippets/%s", template.Storage, template.Filename)
				content, err := proxmox.DownloadSnippetFileResty(r.Context(), restyClient, activeNode, template.Storage, volid)
				if err != nil {
					log.Error().Err(err).
						Str("storage", template.Storage).
						Str("filename", template.Filename).
						Msg("Failed to fetch template content from Proxmox")
					// Continue without content - template metadata is still useful
				} else {
					response["content"] = content
				}
			} else {
				log.Error().Msg("No node information available")
				// Continue without content
			}
		}
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

func (h *CloudInitHandler) redirectWithError(w http.ResponseWriter, r *http.Request, errorKey string) {
	http.Redirect(w, r, "/admin/cloudinit?error="+url.QueryEscape(errorKey), http.StatusSeeOther)
}

func (h *CloudInitHandler) getUsername(r *http.Request) string {
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			return user
		}
	}
	return ""
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
