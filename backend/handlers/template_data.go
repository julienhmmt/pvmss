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

// TemplateOption is a functional option for configuring TemplateData
type TemplateOption func(*TemplateData)

// WithPageType sets the page type (admin, user, public)
func WithPageType(pageType string) TemplateOption {
	return func(td *TemplateData) {
		td.PageType = pageType
	}
}

// WithAdminActive sets the active admin section for menu highlighting
func WithAdminActive(section string) TemplateOption {
	return func(td *TemplateData) {
		td.AdminActive = section
		td.PageType = "admin"
	}
}

// WithAuth sets authentication information from request
func WithAuth(r *http.Request) TemplateOption {
	return func(td *TemplateData) {
		td.IsAuthenticated = IsAuthenticated(r)
		td.IsAdmin = IsAdmin(r)
		if td.IsAuthenticated {
			// Extract username from session if available
			ctx := &HandlerContext{Request: r}
			if username := ctx.GetUsername(); username != "" {
				td.Username = username
			}
		}
	}
}

// WithProxmoxStatus sets Proxmox connection status
func WithProxmoxStatus(sm state.StateManager) TemplateOption {
	return func(td *TemplateData) {
		connected, msg := sm.GetProxmoxStatus()
		td.ProxmoxConnected = connected
		if !connected && msg != "" {
			td.ProxmoxError = msg
		}
	}
}

// WithMessages parses success/error messages from query parameters
func WithMessages(r *http.Request) TemplateOption {
	return func(td *TemplateData) {
		query := r.URL.Query()

		// Success message
		if query.Get("success") != "" {
			td.Success = true
			if msg := query.Get("success_msg"); msg != "" {
				td.SuccessMessage = msg
			}
		}

		// Warning message
		if query.Get("warning") != "" {
			td.Warning = true
			if msg := query.Get("warning_msg"); msg != "" {
				td.WarningMessage = msg
			}
		}

		// Error message
		if query.Get("error") != "" {
			td.Error = true
			if msg := query.Get("error_msg"); msg != "" {
				td.ErrorMessage = msg
			}
		}
	}
}

// WithSuccess sets a success message
func WithSuccess(message string) TemplateOption {
	return func(td *TemplateData) {
		td.Success = true
		td.SuccessMessage = message
	}
}

// WithWarning sets a warning message
func WithWarning(message string) TemplateOption {
	return func(td *TemplateData) {
		td.Warning = true
		td.WarningMessage = message
	}
}

// WithError sets an error message
func WithError(message string) TemplateOption {
	return func(td *TemplateData) {
		td.Error = true
		td.ErrorMessage = message
	}
}

// WithData adds page-specific data
func WithData(key string, value interface{}) TemplateOption {
	return func(td *TemplateData) {
		td.Data[key] = value
	}
}

// WithBreadcrumb adds a breadcrumb item
func WithBreadcrumb(text, url string) TemplateOption {
	return func(td *TemplateData) {
		if td.Breadcrumbs == nil {
			td.Breadcrumbs = make([]BreadcrumbItem, 0)
		}
		td.Breadcrumbs = append(td.Breadcrumbs, BreadcrumbItem{
			Text: text,
			URL:  url,
		})
	}
}

// WithAction adds an action button
func WithAction(text, url string, primary bool) TemplateOption {
	return func(td *TemplateData) {
		if td.Actions == nil {
			td.Actions = make([]ActionButton, 0)
		}
		td.Actions = append(td.Actions, ActionButton{
			Text:    text,
			URL:     url,
			Primary: primary,
		})
	}
}

// NewTemplateDataWithOptions creates a new TemplateData with functional options
// Usage: NewTemplateDataWithOptions("Title", WithAdminActive("section"), WithAuth(r), WithProxmoxStatus(sm)).ToMap()
func NewTemplateDataWithOptions(title string, opts ...TemplateOption) *TemplateData {
	td := &TemplateData{
		Title: title,
		Data:  make(map[string]interface{}),
	}

	// Apply all options
	for _, opt := range opts {
		opt(td)
	}

	return td
}

// NewTemplateData creates a new TemplateDataBuilder for method chaining (backward compatible)
// DEPRECATED: use NewTemplateDataWithOptions with functional options instead
// Usage: NewTemplateData("Title").SetAdminActive("section").SetAuth(r).SetProxmoxStatus(sm).Build().ToMap()
func NewTemplateData(title string) *TemplateDataBuilder {
	return &TemplateDataBuilder{
		data: &TemplateData{
			Title: title,
			Data:  make(map[string]interface{}),
		},
	}
}

// TemplateDataBuilder helps build TemplateData consistently (DEPRECATED: use functional options instead)
type TemplateDataBuilder struct {
	data *TemplateData
}

// NewTemplateDataBuilder creates a new TemplateDataBuilder (DEPRECATED: use NewTemplateData with options instead)
func NewTemplateDataBuilder(title string) *TemplateDataBuilder {
	return &TemplateDataBuilder{
		data: &TemplateData{
			Title: title,
			Data:  make(map[string]interface{}),
		},
	}
}

// SetPageType sets the page type (admin, user, public) (DEPRECATED: use WithPageType option instead)
func (b *TemplateDataBuilder) SetPageType(pageType string) *TemplateDataBuilder {
	b.data.PageType = pageType
	return b
}

// SetAdminActive sets the active admin section for menu highlighting (DEPRECATED: use WithAdminActive option instead)
func (b *TemplateDataBuilder) SetAdminActive(section string) *TemplateDataBuilder {
	b.data.AdminActive = section
	b.data.PageType = "admin"
	return b
}

// SetAuth sets authentication information from request (DEPRECATED: use WithAuth option instead)
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

// SetProxmoxStatus sets Proxmox connection status (DEPRECATED: use WithProxmoxStatus option instead)
func (b *TemplateDataBuilder) SetProxmoxStatus(sm state.StateManager) *TemplateDataBuilder {
	connected, msg := sm.GetProxmoxStatus()
	b.data.ProxmoxConnected = connected
	if !connected && msg != "" {
		b.data.ProxmoxError = msg
	}
	return b
}

// ParseMessages parses success/error messages from query parameters (DEPRECATED: use WithMessages option instead)
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

// SetSuccess sets a success message (DEPRECATED: use WithSuccess option instead)
func (b *TemplateDataBuilder) SetSuccess(message string) *TemplateDataBuilder {
	b.data.Success = true
	b.data.SuccessMessage = message
	return b
}

// SetWarning sets a warning message (DEPRECATED: use WithWarning option instead)
func (b *TemplateDataBuilder) SetWarning(message string) *TemplateDataBuilder {
	b.data.Warning = true
	b.data.WarningMessage = message
	return b
}

// SetError sets an error message (DEPRECATED: use WithError option instead)
func (b *TemplateDataBuilder) SetError(message string) *TemplateDataBuilder {
	b.data.Error = true
	b.data.ErrorMessage = message
	return b
}

// AddData adds page-specific data (DEPRECATED: use WithData option instead)
func (b *TemplateDataBuilder) AddData(key string, value interface{}) *TemplateDataBuilder {
	b.data.Data[key] = value
	return b
}

// AddBreadcrumb adds a breadcrumb item (DEPRECATED: use WithBreadcrumb option instead)
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

// AddAction adds an action button (DEPRECATED: use WithAction option instead)
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

// Build returns the final TemplateData (DEPRECATED: use NewTemplateData with options instead)
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
	u, err := url.Parse(basePath)
	if err != nil {
		// Return basePath as-is if parsing fails
		return basePath
	}
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
	u, err := url.Parse(basePath)
	if err != nil {
		// Return basePath as-is if parsing fails
		return basePath
	}
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
	u, err := url.Parse(basePath)
	if err != nil {
		// Return basePath as-is if parsing fails
		return basePath
	}
	q := u.Query()

	q.Set("warning", "1")
	if message != "" {
		q.Set("warning_msg", message)
	}

	u.RawQuery = q.Encode()
	return u.String()
}
