package handlers

import (
	"fmt"
	"net/http"
	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"

	i18n_bundle "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/zerolog"
)

// ValidationHelper provides common validation methods for handlers
type ValidationHelper struct {
	Log       zerolog.Logger
	Writer    http.ResponseWriter
	Request   *http.Request
	Localizer *i18n_bundle.Localizer
}

// NewValidationHelper creates a new validation helper
func NewValidationHelper(w http.ResponseWriter, r *http.Request, log zerolog.Logger) *ValidationHelper {
	return &ValidationHelper{
		Log:       log,
		Writer:    w,
		Request:   r,
		Localizer: i18n.GetLocalizerFromRequest(r),
	}
}

// RequireProxmoxClient validates and returns Proxmox client or handles error
func (v *ValidationHelper) RequireProxmoxClient(sm state.StateManager) proxmox.ClientInterface {
	if sm == nil {
		v.Log.Error().Msg("State manager is nil")
		http.Error(v.Writer, i18n.Localize(v.Localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return nil
	}

	client := sm.GetProxmoxClient()
	if client == nil {
		v.Log.Error().Msg("Proxmox client not available")
		http.Error(v.Writer, i18n.Localize(v.Localizer, "Proxmox.ConnectionError"), http.StatusServiceUnavailable)
		return nil
	}

	return client
}

// RequireProxmoxConnection validates Proxmox connection with detailed status
func (v *ValidationHelper) RequireProxmoxConnection(sm state.StateManager) (proxmox.ClientInterface, bool) {
	connected, msg := sm.GetProxmoxStatus()
	if !connected {
		v.Log.Warn().Str("proxmox_status", msg).Msg("Proxmox not connected")
		http.Error(v.Writer, fmt.Sprintf("Proxmox connection error: %s", msg), http.StatusServiceUnavailable)
		return nil, false
	}

	client := sm.GetProxmoxClient()
	if client == nil {
		v.Log.Error().Msg("Proxmox client is nil despite connection status")
		http.Error(v.Writer, "Proxmox client unavailable", http.StatusServiceUnavailable)
		return nil, false
	}

	return client, true
}

// RequireFormParsed validates form parsing
func (v *ValidationHelper) RequireFormParsed() bool {
	if err := v.Request.ParseForm(); err != nil {
		v.Log.Error().Err(err).Msg("Failed to parse form")
		RenderErrorPage(v.Writer, v.Request, http.StatusBadRequest, "Invalid form data")
		return false
	}
	return true
}

// RequireMethod validates HTTP method
func (v *ValidationHelper) RequireMethod(method string) bool {
	if v.Request.Method != method {
		v.Log.Warn().Str("expected", method).Str("got", v.Request.Method).Msg("Invalid HTTP method")
		RenderErrorPage(v.Writer, v.Request, http.StatusMethodNotAllowed, "Method not allowed")
		return false
	}
	return true
}

// RequireMethodAndForm validates both HTTP method and form parsing
func (v *ValidationHelper) RequireMethodAndForm(method string) bool {
	return v.RequireMethod(method) && v.RequireFormParsed()
}

// RequireAuthentication validates user authentication
func (v *ValidationHelper) RequireAuthentication() bool {
	session := security.GetSession(v.Request)
	if session == nil {
		v.Log.Error().Msg("Session manager not available")
		http.Redirect(v.Writer, v.Request, "/login", http.StatusSeeOther)
		return false
	}

	authenticated, ok := session.Get(v.Request.Context(), "authenticated").(bool)
	if !ok || !authenticated {
		v.Log.Info().Msg("User not authenticated, redirecting to login")
		returnURL := v.Request.URL.Path
		if v.Request.URL.RawQuery != "" {
			returnURL += "?" + v.Request.URL.RawQuery
		}
		http.Redirect(v.Writer, v.Request, "/login?return="+returnURL, http.StatusSeeOther)
		return false
	}

	return true
}

// RequireAdmin validates admin authentication
func (v *ValidationHelper) RequireAdmin() bool {
	session := security.GetSession(v.Request)
	if session == nil {
		v.Log.Error().Msg("Session manager not available")
		http.Redirect(v.Writer, v.Request, "/admin/login", http.StatusSeeOther)
		return false
	}

	isAdmin, ok := session.Get(v.Request.Context(), "is_admin").(bool)
	if !ok || !isAdmin {
		authenticated, _ := session.Get(v.Request.Context(), "authenticated").(bool)
		if authenticated {
			v.Log.Warn().Msg("Non-admin user attempted to access admin area")
			http.Error(v.Writer, "Access Denied: Admin privileges required", http.StatusForbidden)
		} else {
			v.Log.Info().Msg("Unauthenticated admin access attempt, redirecting")
			returnURL := v.Request.URL.Path
			if v.Request.URL.RawQuery != "" {
				returnURL += "?" + v.Request.URL.RawQuery
			}
			http.Redirect(v.Writer, v.Request, "/admin/login?return="+returnURL, http.StatusSeeOther)
		}
		return false
	}

	return true
}

// RequireFormValues validates that required form values are present
func (v *ValidationHelper) RequireFormValues(fields ...string) (map[string]string, bool) {
	values := make(map[string]string)
	missing := []string{}

	for _, field := range fields {
		value := v.Request.FormValue(field)
		if value == "" {
			missing = append(missing, field)
		}
		values[field] = value
	}

	if len(missing) > 0 {
		v.Log.Warn().Strs("missing_fields", missing).Msg("Required form fields missing")
		http.Error(v.Writer, fmt.Sprintf("Missing required fields: %v", missing), http.StatusBadRequest)
		return nil, false
	}

	return values, true
}
