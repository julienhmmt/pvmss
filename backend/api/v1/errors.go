package apiv1

import (
	"encoding/json"
	stderrors "errors"
	"net/http"

	apperrors "pvmss/errors"
	"pvmss/logger"
)

// writeError writes a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: code, Message: message})
}

// writeJSON writes any value as a JSON response with HTTP 200.
func writeJSON(w http.ResponseWriter, v interface{}) {
	writeJSONStatus(w, http.StatusOK, v)
}

// writeJSONStatus writes any value as a JSON response with the given status code.
// Content-Type is set before WriteHeader to ensure headers are sent correctly.
func writeJSONStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errUnauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
}
func errForbidden(w http.ResponseWriter) {
	writeError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
}
func errBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "bad_request", msg)
}
func errInternal(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}
func errNotFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, "not_found", msg)
}
func errOffline(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "proxmox_offline", "Proxmox is unavailable")
}
func errNotConfigured(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "not_configured", "API auth is not configured (JWT_SECRET environment variable missing or too short)")
}

// codeToHTTPStatus maps domain error codes to HTTP status codes.
func codeToHTTPStatus(code apperrors.ErrorCode) int {
	switch code {
	case apperrors.CodeNotFound:
		return http.StatusNotFound
	case apperrors.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperrors.CodeForbidden:
		return http.StatusForbidden
	case apperrors.CodeValidation:
		return http.StatusBadRequest
	case apperrors.CodeConflict:
		return http.StatusConflict
	case apperrors.CodeRateLimited:
		return http.StatusTooManyRequests
	case apperrors.CodeTimeout:
		return http.StatusGatewayTimeout
	case apperrors.CodeUnavailable:
		return http.StatusServiceUnavailable
	case apperrors.CodeNotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

// wireCodeFor maps a domain ErrorCode to the lowercase wire-format code used
// by the existing api/v1 error helpers. Keeps the JSON shape stable for
// clients that may key off `code` strings.
func wireCodeFor(code apperrors.ErrorCode) string {
	switch code {
	case apperrors.CodeNotFound:
		return "not_found"
	case apperrors.CodeUnauthorized:
		return "unauthorized"
	case apperrors.CodeForbidden:
		return "forbidden"
	case apperrors.CodeValidation:
		return "bad_request"
	case apperrors.CodeConflict:
		return "conflict"
	case apperrors.CodeRateLimited:
		return "rate_limited"
	case apperrors.CodeTimeout:
		return "timeout"
	case apperrors.CodeUnavailable:
		return "service_unavailable"
	case apperrors.CodeNotImplemented:
		return "not_implemented"
	default:
		return "internal_error"
	}
}

// writeAppError logs err once and writes a JSON error response. The status
// code is derived from the wrapped *errors.AppError (or specialised subtype);
// errors with no AppError in their chain map to 500. Use this instead of
// pairing log.Error() with errInternal(w) / errBadRequest(w) at handler sites.
func writeAppError(w http.ResponseWriter, err error) {
	if err == nil {
		errInternal(w)
		return
	}

	code := apperrors.GetCode(err)
	status := codeToHTTPStatus(code)
	var proxmoxErr *apperrors.ProxmoxError
	if stderrors.As(err, &proxmoxErr) && isHTTPErrorStatus(proxmoxErr.StatusCode) {
		status = proxmoxErr.StatusCode
	}

	logger.Get().Error().
		Err(err).
		Str("code", string(code)).
		Int("status", status).
		Msg("handler error")

	msg := publicMessage(err, status)
	writeError(w, status, wireCodeFor(code), msg)
}

func isHTTPErrorStatus(status int) bool {
	return status >= http.StatusBadRequest && status <= http.StatusNetworkAuthenticationRequired
}

// publicMessage returns a client-safe message. For 5xx we never leak the
// underlying error; for 4xx we surface the AppError.Message when available.
func publicMessage(err error, status int) string {
	if status >= 500 {
		return "Internal server error"
	}
	if msg := appErrorMessage(err); msg != "" {
		return msg
	}
	if msg := http.StatusText(status); msg != "" {
		return msg
	}
	return "Request failed"
}

func appErrorMessage(err error) string {
	var validationErr *apperrors.ValidationError
	if stderrors.As(err, &validationErr) {
		return validationErr.Message
	}
	var proxmoxErr *apperrors.ProxmoxError
	if stderrors.As(err, &proxmoxErr) {
		return proxmoxErr.Message
	}
	var vmErr *apperrors.VMError
	if stderrors.As(err, &vmErr) {
		return vmErr.Message
	}
	var authErr *apperrors.AuthError
	if stderrors.As(err, &authErr) {
		return authErr.Message
	}
	var configErr *apperrors.ConfigError
	if stderrors.As(err, &configErr) {
		return configErr.Message
	}
	var appErr *apperrors.AppError
	if stderrors.As(err, &appErr) {
		return appErr.Message
	}
	return ""
}
