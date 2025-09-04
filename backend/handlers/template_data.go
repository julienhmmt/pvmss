package handlers

import (
	"net/http"
	"net/url"

	"pvmss/state"
)

// TemplateData represents standardized data structure for template rendering
type TemplateData struct {
	// Core page information
	Title       string `json:"title"`
	PageType    string `json:"page_type"`              // "admin", "user", "public"
	AdminActive string `json:"admin_active,omitempty"` // For admin menu highlighting

	// Authentication and user context
	IsAuthenticated bool   `json:"is_authenticated"`
	IsAdmin         bool   `json:"is_admin"`
	Username        string `json:"username,omitempty"`

	// Status and messaging
	Success        bool   `json:"success"`
	SuccessMessage string `json:"success_message,omitempty"`
	Warning        bool   `json:"warning"`
	WarningMessage string `json:"warning_message,omitempty"`
	Error          bool   `json:"error"`
	ErrorMessage   string `json:"error_message,omitempty"`

	// Proxmox connection status
	ProxmoxConnected bool   `json:"proxmox_connected"`
	ProxmoxError     string `json:"proxmox_error,omitempty"`

	// Page-specific data
	Data map[string]interface{} `json:"data,omitempty"`

	// Navigation and UI state
	ActiveSection string           `json:"active_section,omitempty"`
	Breadcrumbs   []BreadcrumbItem `json:"breadcrumbs,omitempty"`
	Actions       []ActionButton   `json:"actions,omitempty"`
}

// BreadcrumbItem represents a breadcrumb navigation item
type BreadcrumbItem struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"` // Empty for current page
}

// ActionButton represents an action button in the UI
type ActionButton struct {
	Text    string `json:"text"`
	URL     string `json:"url,omitempty"`
	Action  string `json:"action,omitempty"` // For JS actions
	Primary bool   `json:"primary"`
	Icon    string `json:"icon,omitempty"`
}

// TemplateDataBuilder helps build TemplateData consistently
type TemplateDataBuilder struct {
	data *TemplateData
}

// NewTemplateData creates a new TemplateDataBuilder
func NewTemplateData(title string) *TemplateDataBuilder {
	return &TemplateDataBuilder{
		data: &TemplateData{
			Title: title,
			Data:  make(map[string]interface{}),
		},
	}
}

// SetPageType sets the page type (admin, user, public)
func (b *TemplateDataBuilder) SetPageType(pageType string) *TemplateDataBuilder {
	b.data.PageType = pageType
	return b
}

// SetAdminActive sets the active admin section for menu highlighting
func (b *TemplateDataBuilder) SetAdminActive(section string) *TemplateDataBuilder {
	b.data.AdminActive = section
	b.data.PageType = "admin"
	return b
}

// SetAuth sets authentication information from request
func (b *TemplateDataBuilder) SetAuth(r *http.Request) *TemplateDataBuilder {
	b.data.IsAuthenticated = IsAuthenticated(r)
	b.data.IsAdmin = IsAdmin(r)
	if b.data.IsAuthenticated {
		// Extract username from session if available
		ctx := &HandlerContext{Request: r}
		if username := ctx.GetUsername(); username != "" {
			b.data.Username = username
		}
	}
	return b
}

// SetProxmoxStatus sets Proxmox connection status
func (b *TemplateDataBuilder) SetProxmoxStatus(sm state.StateManager) *TemplateDataBuilder {
	connected, msg := sm.GetProxmoxStatus()
	b.data.ProxmoxConnected = connected
	if !connected && msg != "" {
		b.data.ProxmoxError = msg
	}
	return b
}

// ParseMessages parses success/error messages from query parameters
func (b *TemplateDataBuilder) ParseMessages(r *http.Request) *TemplateDataBuilder {
	query := r.URL.Query()

	// Success message
	if query.Get("success") != "" {
		b.data.Success = true
		if msg := query.Get("success_msg"); msg != "" {
			b.data.SuccessMessage = msg
		}
	}

	// Warning message
	if query.Get("warning") != "" {
		b.data.Warning = true
		if msg := query.Get("warning_msg"); msg != "" {
			b.data.WarningMessage = msg
		}
	}

	// Error message
	if query.Get("error") != "" {
		b.data.Error = true
		if msg := query.Get("error_msg"); msg != "" {
			b.data.ErrorMessage = msg
		}
	}

	return b
}

// SetSuccess sets a success message
func (b *TemplateDataBuilder) SetSuccess(message string) *TemplateDataBuilder {
	b.data.Success = true
	b.data.SuccessMessage = message
	return b
}

// SetWarning sets a warning message
func (b *TemplateDataBuilder) SetWarning(message string) *TemplateDataBuilder {
	b.data.Warning = true
	b.data.WarningMessage = message
	return b
}

// SetError sets an error message
func (b *TemplateDataBuilder) SetError(message string) *TemplateDataBuilder {
	b.data.Error = true
	b.data.ErrorMessage = message
	return b
}

// AddData adds page-specific data
func (b *TemplateDataBuilder) AddData(key string, value interface{}) *TemplateDataBuilder {
	b.data.Data[key] = value
	return b
}

// AddBreadcrumb adds a breadcrumb item
func (b *TemplateDataBuilder) AddBreadcrumb(text, url string) *TemplateDataBuilder {
	if b.data.Breadcrumbs == nil {
		b.data.Breadcrumbs = make([]BreadcrumbItem, 0)
	}
	b.data.Breadcrumbs = append(b.data.Breadcrumbs, BreadcrumbItem{
		Text: text,
		URL:  url,
	})
	return b
}

// AddAction adds an action button
func (b *TemplateDataBuilder) AddAction(text, url string, primary bool) *TemplateDataBuilder {
	if b.data.Actions == nil {
		b.data.Actions = make([]ActionButton, 0)
	}
	b.data.Actions = append(b.data.Actions, ActionButton{
		Text:    text,
		URL:     url,
		Primary: primary,
	})
	return b
}

// Build returns the final TemplateData
func (b *TemplateDataBuilder) Build() *TemplateData {
	return b.data
}

// ToMap converts TemplateData to map[string]interface{} for template rendering
func (td *TemplateData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"Title":            td.Title,
		"PageType":         td.PageType,
		"IsAuthenticated":  td.IsAuthenticated,
		"IsAdmin":          td.IsAdmin,
		"Success":          td.Success,
		"Warning":          td.Warning,
		"Error":            td.Error,
		"ProxmoxConnected": td.ProxmoxConnected,
	}

	// Add optional fields only if they have values
	if td.AdminActive != "" {
		result["AdminActive"] = td.AdminActive
	}
	if td.Username != "" {
		result["Username"] = td.Username
	}
	if td.SuccessMessage != "" {
		result["SuccessMessage"] = td.SuccessMessage
	}
	if td.WarningMessage != "" {
		result["WarningMessage"] = td.WarningMessage
	}
	if td.ErrorMessage != "" {
		result["ErrorMessage"] = td.ErrorMessage
	}
	if td.ProxmoxError != "" {
		result["ProxmoxError"] = td.ProxmoxError
	}
	if td.ActiveSection != "" {
		result["ActiveSection"] = td.ActiveSection
	}
	if len(td.Breadcrumbs) > 0 {
		result["Breadcrumbs"] = td.Breadcrumbs
	}
	if len(td.Actions) > 0 {
		result["Actions"] = td.Actions
	}

	// Add page-specific data
	for k, v := range td.Data {
		result[k] = v
	}

	return result
}

// MessageHelper provides utilities for standardized message handling
type MessageHelper struct{}

// NewMessageHelper creates a new MessageHelper
func NewMessageHelper() *MessageHelper {
	return &MessageHelper{}
}

// BuildSuccessURL builds a URL with success parameters
func (m *MessageHelper) BuildSuccessURL(basePath string, message string, params map[string]string) string {
	u, _ := url.Parse(basePath)
	q := u.Query()

	q.Set("success", "1")
	if message != "" {
		q.Set("success_msg", message)
	}

	for key, value := range params {
		q.Set(key, value)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// BuildErrorURL builds a URL with error parameters
func (m *MessageHelper) BuildErrorURL(basePath string, message string) string {
	u, _ := url.Parse(basePath)
	q := u.Query()

	q.Set("error", "1")
	if message != "" {
		q.Set("error_msg", message)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// BuildWarningURL builds a URL with warning parameters
func (m *MessageHelper) BuildWarningURL(basePath string, message string) string {
	u, _ := url.Parse(basePath)
	q := u.Query()

	q.Set("warning", "1")
	if message != "" {
		q.Set("warning_msg", message)
	}

	u.RawQuery = q.Encode()
	return u.String()
}
