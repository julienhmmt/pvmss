package handlers

import (
	"net/http"
	"pvmss/i18n"
	"pvmss/logger"

	i18n_bundle "github.com/nicksnyder/go-i18n/v2/i18n"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Code    int
	Message string
	Key     string
}

// Common error responses with i18n keys
var (
	ErrUnauthorized = ErrorResponse{
		Code:    http.StatusUnauthorized,
		Key:     "Error.Unauthorized",
		Message: "Unauthorized",
	}
	ErrForbidden = ErrorResponse{
		Code:    http.StatusForbidden,
		Key:     "Error.Forbidden",
		Message: "Access denied",
	}
	ErrNotFound = ErrorResponse{
		Code:    http.StatusNotFound,
		Key:     "Error.NotFound",
		Message: "Resource not found",
	}
	ErrBadRequest = ErrorResponse{
		Code:    http.StatusBadRequest,
		Key:     "Error.BadRequest",
		Message: "Invalid request",
	}
	ErrInternalServer = ErrorResponse{
		Code:    http.StatusInternalServerError,
		Key:     "Error.InternalServer",
		Message: "Internal server error",
	}
	ErrServiceUnavailable = ErrorResponse{
		Code:    http.StatusServiceUnavailable,
		Key:     "Error.ServiceUnavailable",
		Message: "Service temporarily unavailable",
	}
	ErrMethodNotAllowed = ErrorResponse{
		Code:    http.StatusMethodNotAllowed,
		Key:     "Error.MethodNotAllowed",
		Message: "Method not allowed",
	}
	ErrProxmoxConnection = ErrorResponse{
		Code:    http.StatusServiceUnavailable,
		Key:     "Proxmox.ConnectionError",
		Message: "Unable to connect to Proxmox server",
	}
	ErrSessionExpired = ErrorResponse{
		Code:    http.StatusUnauthorized,
		Key:     "Error.SessionExpired",
		Message: "Session expired, please log in again",
	}
	ErrInvalidCredentials = ErrorResponse{
		Code:    http.StatusUnauthorized,
		Key:     "Error.InvalidCredentials",
		Message: "Invalid credentials",
	}
	ErrCSRFValidation = ErrorResponse{
		Code:    http.StatusBadRequest,
		Key:     "Error.CSRFValidation",
		Message: "Invalid request. Please try again.",
	}
)

// RespondWithError sends a standardized error response with i18n support
func RespondWithError(w http.ResponseWriter, r *http.Request, errResp ErrorResponse) {
	localizer := i18n.GetLocalizerFromRequest(r)
	message := i18n.Localize(localizer, errResp.Key)
	if message == "" {
		message = errResp.Message
	}

	logger.Get().Warn().
		Int("status_code", errResp.Code).
		Str("error_key", errResp.Key).
		Str("path", r.URL.Path).
		Msg("Error response sent")

	http.Error(w, message, errResp.Code)
}

// RespondWithErrorAndLog sends error response and logs with additional context
func RespondWithErrorAndLog(w http.ResponseWriter, r *http.Request, errResp ErrorResponse, err error, context string) {
	localizer := i18n.GetLocalizerFromRequest(r)
	message := i18n.Localize(localizer, errResp.Key)
	if message == "" {
		message = errResp.Message
	}

	logger.Get().Error().
		Err(err).
		Int("status_code", errResp.Code).
		Str("error_key", errResp.Key).
		Str("context", context).
		Str("path", r.URL.Path).
		Msg("Error occurred")

	http.Error(w, message, errResp.Code)
}

// RespondWithCustomError sends a custom error message with i18n key
func RespondWithCustomError(w http.ResponseWriter, r *http.Request, statusCode int, i18nKey string, fallbackMsg string) {
	localizer := i18n.GetLocalizerFromRequest(r)
	message := i18n.Localize(localizer, i18nKey)
	if message == "" {
		message = fallbackMsg
	}

	logger.Get().Warn().
		Int("status_code", statusCode).
		Str("error_key", i18nKey).
		Str("path", r.URL.Path).
		Msg("Custom error response sent")

	http.Error(w, message, statusCode)
}

// RenderErrorPageWithI18n renders an error page with i18n support
func RenderErrorPageWithI18n(w http.ResponseWriter, r *http.Request, statusCode int, i18nKey string, fallbackMsg string) {
	localizer := i18n.GetLocalizerFromRequest(r)
	message := i18n.Localize(localizer, i18nKey)
	if message == "" {
		message = fallbackMsg
	}

	RenderErrorPage(w, r, statusCode, message)
}

// LocalizeError translates an error key to the user's language
func LocalizeError(r *http.Request, key string) string {
	localizer := i18n.GetLocalizerFromRequest(r)
	return i18n.Localize(localizer, key)
}

// LocalizeErrorWithFallback translates an error key with a fallback message
func LocalizeErrorWithFallback(r *http.Request, key string, fallback string) string {
	localizer := i18n.GetLocalizerFromRequest(r)
	msg := i18n.Localize(localizer, key)
	if msg == "" {
		return fallback
	}
	return msg
}

// ErrorHelper provides centralized error handling with i18n
type ErrorHelper struct {
	Writer    http.ResponseWriter
	Request   *http.Request
	Localizer *i18n_bundle.Localizer
}

// NewErrorHelper creates a new error helper
func NewErrorHelper(w http.ResponseWriter, r *http.Request) *ErrorHelper {
	return &ErrorHelper{
		Writer:    w,
		Request:   r,
		Localizer: i18n.GetLocalizerFromRequest(r),
	}
}

// Send sends a standardized error response
func (e *ErrorHelper) Send(errResp ErrorResponse) {
	RespondWithError(e.Writer, e.Request, errResp)
}

// SendWithLog sends error response with logging
func (e *ErrorHelper) SendWithLog(errResp ErrorResponse, err error, context string) {
	RespondWithErrorAndLog(e.Writer, e.Request, errResp, err, context)
}

// SendCustom sends a custom error message
func (e *ErrorHelper) SendCustom(statusCode int, i18nKey string, fallbackMsg string) {
	RespondWithCustomError(e.Writer, e.Request, statusCode, i18nKey, fallbackMsg)
}

// RenderPage renders an error page
func (e *ErrorHelper) RenderPage(statusCode int, i18nKey string, fallbackMsg string) {
	RenderErrorPageWithI18n(e.Writer, e.Request, statusCode, i18nKey, fallbackMsg)
}

// Localize translates an i18n key
func (e *ErrorHelper) Localize(key string) string {
	return i18n.Localize(e.Localizer, key)
}
