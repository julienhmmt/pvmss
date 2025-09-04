package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"pvmss/logger"
	"pvmss/state"
)

// BaseAdminHandler provides common admin functionality
type BaseAdminHandler struct {
	stateManager state.StateManager
}

// NewBaseAdminHandler creates a new BaseAdminHandler
func NewBaseAdminHandler(sm state.StateManager) *BaseAdminHandler {
	return &BaseAdminHandler{stateManager: sm}
}

// GetStateManager returns the state manager
func (b *BaseAdminHandler) GetStateManager() state.StateManager {
	return b.stateManager
}

// CreateLogger creates a handler logger with standard naming
func (b *BaseAdminHandler) CreateLogger(handlerName string, r *http.Request) zerolog.Logger {
	return CreateHandlerLogger(handlerName, r)
}

// ValidateAdminAccess checks if the user has admin access
func (b *BaseAdminHandler) ValidateAdminAccess(w http.ResponseWriter, r *http.Request) bool {
	if !IsAdmin(r) {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return false
	}
	return true
}

// GetProxmoxStatus returns proxmox connection status and client
func (b *BaseAdminHandler) GetProxmoxStatus() (bool, interface{}, string) {
	connected, msg := b.stateManager.GetProxmoxStatus()
	client := b.stateManager.GetProxmoxClient()
	return connected, client, msg
}

// BaseFormHandler provides common form processing functionality
type BaseFormHandler struct {
	*BaseAdminHandler
}

// NewBaseFormHandler creates a new BaseFormHandler
func NewBaseFormHandler(sm state.StateManager) *BaseFormHandler {
	return &BaseFormHandler{
		BaseAdminHandler: NewBaseAdminHandler(sm),
	}
}

// ValidateForm validates common form requirements
func (b *BaseFormHandler) ValidateForm(w http.ResponseWriter, r *http.Request) bool {
	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return false
	}
	return true
}

// ParseIntField safely parses an integer field from form data
func (b *BaseFormHandler) ParseIntField(r *http.Request, fieldName string, fallback int) int {
	v := r.FormValue(fieldName)
	if v == "" {
		return fallback
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return fallback
}

// ParseBoolField safely parses a boolean field from form data
func (b *BaseFormHandler) ParseBoolField(r *http.Request, fieldName string) bool {
	v := strings.ToLower(r.FormValue(fieldName))
	return v == "true" || v == "1" || v == "on" || v == "yes"
}

// ValidateRequiredFields checks that required fields are not empty
func (b *BaseFormHandler) ValidateRequiredFields(w http.ResponseWriter, r *http.Request, fields []string) bool {
	for _, field := range fields {
		if strings.TrimSpace(r.FormValue(field)) == "" {
			http.Error(w, "Missing required field: "+field, http.StatusBadRequest)
			return false
		}
	}
	return true
}

// RedirectWithSuccess redirects to a path with success parameters
func (b *BaseFormHandler) RedirectWithSuccess(w http.ResponseWriter, r *http.Request, path string, params map[string]string) {
	u, _ := url.Parse(path)
	q := u.Query()
	q.Set("success", "1")

	for key, value := range params {
		q.Set(key, value)
	}

	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// BaseAPIHandler provides common API response patterns
type BaseAPIHandler struct {
	*BaseAdminHandler
}

// NewBaseAPIHandler creates a new BaseAPIHandler
func NewBaseAPIHandler(sm state.StateManager) *BaseAPIHandler {
	return &BaseAPIHandler{
		BaseAdminHandler: NewBaseAdminHandler(sm),
	}
}

// WriteJSONResponse writes a JSON response with proper headers
func (b *BaseAPIHandler) WriteJSONResponse(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

// WriteJSONError writes a JSON error response
func (b *BaseAPIHandler) WriteJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "error",
		"message": message,
	})
}

// WriteJSONSuccess writes a JSON success response
func (b *BaseAPIHandler) WriteJSONSuccess(w http.ResponseWriter, data interface{}) {
	response := map[string]interface{}{
		"status": "success",
	}

	if data != nil {
		if dataMap, ok := data.(map[string]interface{}); ok {
			for k, v := range dataMap {
				response[k] = v
			}
		} else {
			response["data"] = data
		}
	}

	b.WriteJSONResponse(w, response)
}

// ValidateAPIAccess checks API access requirements
func (b *BaseAPIHandler) ValidateAPIAccess(w http.ResponseWriter, r *http.Request, requireAdmin bool) bool {
	if requireAdmin {
		if !IsAdmin(r) {
			b.WriteJSONError(w, "Admin access required", http.StatusForbidden)
			return false
		}
	} else {
		if !IsAuthenticated(r) {
			b.WriteJSONError(w, "Authentication required", http.StatusUnauthorized)
			return false
		}
	}
	return true
}

// HandleOfflineMode handles cases when Proxmox is offline
func (b *BaseAPIHandler) HandleOfflineMode(w http.ResponseWriter, fallbackData interface{}) {
	logger.Get().Warn().Msg("Proxmox offline; returning fallback data")
	response := map[string]interface{}{
		"status":  "offline",
		"message": "Proxmox connection unavailable",
	}

	if fallbackData != nil {
		if dataMap, ok := fallbackData.(map[string]interface{}); ok {
			for k, v := range dataMap {
				response[k] = v
			}
		} else {
			response["data"] = fallbackData
		}
	}

	b.WriteJSONResponse(w, response)
}
