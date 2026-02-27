package handlers

import (
	stderrors "errors"
	"net/http"

	"pvmss/errors"
	"pvmss/logger"
)

// ErrorHandler provides standardized error handling for HTTP handlers.
type ErrorHandler struct {
	log logger.Logger
}

// ErrorHandlerWith creates a new error handler with logging.
func ErrorHandlerWith(log logger.Logger) *ErrorHandler {
	return &ErrorHandler{log: log}
}

// HandleError logs an error and writes an appropriate HTTP response.
func (eh *ErrorHandler) HandleError(w http.ResponseWriter, err error, defaultMessage string) {
	if err == nil {
		return
	}

	code := errors.GetCode(err)
	statusCode := codeToHTTPStatus(code)

	eh.log.Error().
		Err(err).
		Str("code", string(code)).
		Int("status", statusCode).
		Msg("Handler error")

	http.Error(w, defaultMessage, statusCode)
}

// HandleErrorWithContext logs an error with additional context and writes an HTTP response.
func (eh *ErrorHandler) HandleErrorWithContext(w http.ResponseWriter, err error, defaultMessage string, context map[string]interface{}) {
	if err == nil {
		return
	}

	code := errors.GetCode(err)
	statusCode := codeToHTTPStatus(code)

	logCtx := eh.log.Error().
		Err(err).
		Str("code", string(code)).
		Int("status", statusCode)

	for k, v := range context {
		logCtx = logCtx.Interface(k, v)
	}

	logCtx.Msg("Handler error with context")

	http.Error(w, defaultMessage, statusCode)
}

// codeToHTTPStatus converts an error code to an HTTP status code.
func codeToHTTPStatus(code errors.ErrorCode) int {
	switch code {
	case errors.CodeNotFound:
		return http.StatusNotFound
	case errors.CodeUnauthorized:
		return http.StatusUnauthorized
	case errors.CodeForbidden:
		return http.StatusForbidden
	case errors.CodeValidation:
		return http.StatusBadRequest
	case errors.CodeConflict:
		return http.StatusConflict
	case errors.CodeRateLimited:
		return http.StatusTooManyRequests
	case errors.CodeTimeout:
		return http.StatusGatewayTimeout
	case errors.CodeUnavailable:
		return http.StatusServiceUnavailable
	case errors.CodeNotImplemented:
		return http.StatusNotImplemented
	case errors.CodeProxmox, errors.CodeVM, errors.CodeAuth, errors.CodeConfig:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// WrapHandlerError wraps a handler error with context.
// Use this for errors that occur during handler execution.
func WrapHandlerError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// If it's already an AppError, wrap it
	var appErr *errors.AppError
	if stderrors.As(err, &appErr) {
		return errors.Wrap(err, appErr.Code, operation)
	}

	// Otherwise, wrap as internal error
	return errors.Wrap(err, errors.CodeInternal, operation)
}

// WrapProxmoxHandlerError wraps a Proxmox API error from a handler.
// Use this for errors from Proxmox API calls.
func WrapProxmoxHandlerError(err error, endpoint string, statusCode int, operation string) error {
	if err == nil {
		return nil
	}

	return errors.WrapProxmox(err, endpoint, statusCode, operation)
}

// WrapValidationError wraps a validation error from a handler.
// Use this for input validation failures.
func WrapValidationError(field string, value interface{}, message string) error {
	return errors.ValidationErr(field, value, message)
}

// LogError logs an error with structured context.
func LogError(log logger.Logger, err error, operation string, context map[string]interface{}) {
	if err == nil {
		return
	}

	code := errors.GetCode(err)
	logCtx := log.Error().
		Err(err).
		Str("code", string(code)).
		Str("operation", operation)

	for k, v := range context {
		logCtx = logCtx.Interface(k, v)
	}

	logCtx.Msg("Operation failed")
}

// LogWarning logs a warning with structured context.
func LogWarning(log logger.Logger, message string, context map[string]interface{}) {
	logCtx := log.Warn().Str("message", message)

	for k, v := range context {
		logCtx = logCtx.Interface(k, v)
	}

	logCtx.Msg("Warning")
}
