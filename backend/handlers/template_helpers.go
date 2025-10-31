package handlers

import (
	"net/http"

	"pvmss/state"
)

// TemplateHelpers provides utility functions for template rendering
type TemplateHelpers struct{}

// NewTemplateHelpers creates a new TemplateHelpers instance
func NewTemplateHelpers() *TemplateHelpers {
	return &TemplateHelpers{}
}

// RenderAdminPage renders an admin page with standardized data structure
func (th *TemplateHelpers) RenderAdminPage(w http.ResponseWriter, r *http.Request, templateName string, title string, adminSection string, sm state.StateManager, customData map[string]interface{}) {
	builder := NewTemplateData(title).
		SetAdminActive(adminSection).
		SetAuth(r).
		SetProxmoxStatus(sm).
		ParseMessages(r)

	// Add custom data
	for key, value := range customData {
		builder.AddData(key, value)
	}

	data := builder.Build().ToMap()
	renderTemplateInternal(w, r, templateName, data)
}

// RenderUserPage renders a user page with standardized data structure
func (th *TemplateHelpers) RenderUserPage(w http.ResponseWriter, r *http.Request, templateName string, title string, sm state.StateManager, customData map[string]interface{}) {
	builder := NewTemplateData(title).
		SetPageType("user").
		SetAuth(r).
		SetProxmoxStatus(sm).
		ParseMessages(r)

	// Add custom data
	for key, value := range customData {
		builder.AddData(key, value)
	}

	data := builder.Build().ToMap()
	renderTemplateInternal(w, r, templateName, data)
}

// RenderPublicPage renders a public page with standardized data structure
func (th *TemplateHelpers) RenderPublicPage(w http.ResponseWriter, r *http.Request, templateName string, title string, customData map[string]interface{}) {
	builder := NewTemplateData(title).
		SetPageType("public").
		ParseMessages(r)

	// Add custom data
	for key, value := range customData {
		builder.AddData(key, value)
	}

	data := builder.Build().ToMap()
	renderTemplateInternal(w, r, templateName, data)
}

// StandardAdminPageData creates standardized admin page data (legacy compatibility)
func StandardAdminPageData(title string, r *http.Request, sm state.StateManager, adminSection string) map[string]interface{} {
	builder := NewTemplateData(title).
		SetAdminActive(adminSection).
		SetAuth(r).
		SetProxmoxStatus(sm).
		ParseMessages(r)

	return builder.Build().ToMap()
}

// StandardUserPageData creates standardized user page data (legacy compatibility)
func StandardUserPageData(title string, r *http.Request, sm state.StateManager) map[string]interface{} {
	builder := NewTemplateData(title).
		SetPageType("user").
		SetAuth(r).
		SetProxmoxStatus(sm).
		ParseMessages(r)

	return builder.Build().ToMap()
}

// MessageHandlers provides standardized message handling
type MessageHandlers struct {
	helper *MessageHelper
}

// NewMessageHandlers creates a new MessageHandlers instance
func NewMessageHandlers() *MessageHandlers {
	return &MessageHandlers{
		helper: NewMessageHelper(),
	}
}

// RedirectWithSuccess redirects with a success message
func (mh *MessageHandlers) RedirectWithSuccess(w http.ResponseWriter, r *http.Request, path string, message string, params map[string]string) {
	url := mh.helper.BuildSuccessURL(path, message, params)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// RedirectWithError redirects with an error message
func (mh *MessageHandlers) RedirectWithError(w http.ResponseWriter, r *http.Request, path string, message string) {
	url := mh.helper.BuildErrorURL(path, message)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// RedirectWithWarning redirects with a warning message
func (mh *MessageHandlers) RedirectWithWarning(w http.ResponseWriter, r *http.Request, path string, message string) {
	url := mh.helper.BuildWarningURL(path, message)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// GenerateStandardSuccessMessages creates common success messages
func (mh *MessageHandlers) GenerateStandardSuccessMessages() map[string]string {
	return map[string]string{
		"created":  "Resource created successfully",
		"updated":  "Resource updated successfully",
		"deleted":  "Resource deleted successfully",
		"enabled":  "Resource enabled successfully",
		"disabled": "Resource disabled successfully",
		"saved":    "Settings saved successfully",
		"imported": "Data imported successfully",
		"exported": "Data exported successfully",
	}
}

// GenerateStandardErrorMessages creates common error messages
func (mh *MessageHandlers) GenerateStandardErrorMessages() map[string]string {
	return map[string]string{
		"not_found":     "Resource not found",
		"forbidden":     "Access denied",
		"invalid_input": "Invalid input data",
		"server_error":  "Internal server error",
		"proxmox_error": "Proxmox connection error",
		"save_failed":   "Failed to save settings",
		"delete_failed": "Failed to delete resource",
		"update_failed": "Failed to update resource",
	}
}

// ContextualMessageHelper provides context-aware message generation
type ContextualMessageHelper struct {
	messages *MessageHandlers
}

// NewContextualMessageHelper creates a new ContextualMessageHelper
func NewContextualMessageHelper() *ContextualMessageHelper {
	return &ContextualMessageHelper{
		messages: NewMessageHandlers(),
	}
}

// GenerateISOMessages generates ISO-specific messages
func (cmh *ContextualMessageHelper) GenerateISOMessages(action string, isoName string) string {
	switch action {
	case "enable":
		return "ISO '" + isoName + "' enabled successfully"
	case "disable":
		return "ISO '" + isoName + "' disabled successfully"
	case "upload":
		return "ISO '" + isoName + "' uploaded successfully"
	case "delete":
		return "ISO '" + isoName + "' deleted successfully"
	default:
		return "ISO operation completed successfully"
	}
}

// GenerateVMMessages generates VM-specific messages
func (cmh *ContextualMessageHelper) GenerateVMMessages(action string, vmName string) string {
	switch action {
	case "create":
		return "VM '" + vmName + "' created successfully"
	case "start":
		return "VM '" + vmName + "' started successfully"
	case "stop":
		return "VM '" + vmName + "' stopped successfully"
	case "restart":
		return "VM '" + vmName + "' restarted successfully"
	case "delete":
		return "VM '" + vmName + "' deleted successfully"
	case "clone":
		return "VM '" + vmName + "' cloned successfully"
	default:
		return "VM operation completed successfully"
	}
}

// GenerateStorageMessages generates storage-specific messages
func (cmh *ContextualMessageHelper) GenerateStorageMessages(action string, storageName string) string {
	switch action {
	case "enable":
		return "Storage '" + storageName + "' enabled successfully"
	case "disable":
		return "Storage '" + storageName + "' disabled successfully"
	case "update":
		return "Storage '" + storageName + "' updated successfully"
	default:
		return "Storage operation completed successfully"
	}
}

// GenerateUserMessages generates user-specific messages
func (cmh *ContextualMessageHelper) GenerateUserMessages(action string, username string) string {
	switch action {
	case "create":
		return "User '" + username + "' created successfully"
	case "update":
		return "User '" + username + "' updated successfully"
	case "delete":
		return "User '" + username + "' deleted successfully"
	case "enable":
		return "User '" + username + "' enabled successfully"
	case "disable":
		return "User '" + username + "' disabled successfully"
	default:
		return "User operation completed successfully"
	}
}

// GenerateNodeMessages generates node-specific messages
func (cmh *ContextualMessageHelper) GenerateNodeMessages(action string, nodeName string) string {
	switch action {
	case "update":
		return "Node '" + nodeName + "' settings updated successfully"
	case "restart":
		return "Node '" + nodeName + "' restart initiated"
	case "shutdown":
		return "Node '" + nodeName + "' shutdown initiated"
	default:
		return "Node operation completed successfully"
	}
}

// AdminPageDataWithSection provides backward compatibility with admin section
func AdminPageDataWithSection(title string, r *http.Request, sm state.StateManager, section string) map[string]interface{} {
	return StandardAdminPageData(title, r, sm, section)
}
