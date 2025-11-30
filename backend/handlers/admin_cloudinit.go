package handlers

import (
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
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateTemplateHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Edit template
	router.POST("/admin/cloudinit/edit", SecureFormHandler("CloudInitEdit",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.EditTemplateHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Delete template
	router.POST("/admin/cloudinit/delete", SecureFormHandler("CloudInitDelete",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.DeleteTemplateHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Toggle template enabled state
	router.POST("/admin/cloudinit/toggle", SecureFormHandler("CloudInitToggle",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.ToggleTemplateHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

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

	// TODO: Upload snippet file to Proxmox storage
	// For now, we just save the metadata locally
	// In production, this would use proxmox.UploadSnippetFileResty

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
func (h *CloudInitHandler) DeleteTemplateHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteTemplateHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.RequiredFields")
		return
	}

	settings := h.stateManager.GetSettings()
	template := settings.GetCloudInitTemplateByID(id)
	if template == nil {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.NotFound")
		return
	}

	templateName := template.Name

	// TODO: Delete snippet file from Proxmox storage
	// For now, we just remove the metadata locally

	if !settings.RemoveCloudInitTemplate(id) {
		h.redirectWithError(w, r, "Admin.CloudInit.Error.DeleteFailed")
		return
	}

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to save settings")
		h.redirectWithError(w, r, "Error.InternalServer")
		return
	}

	// Audit log
	username := h.getUsername(r)
	logger.AdminEvent("cloudinit_template_delete", username).
		Str("template_id", id).
		Str("template_name", templateName).
		Str("client_ip", r.RemoteAddr).
		Msg("Cloud-init template deleted")

	http.Redirect(w, r, "/admin/cloudinit?success=1&action=delete&name="+url.QueryEscape(templateName), http.StatusSeeOther)
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
