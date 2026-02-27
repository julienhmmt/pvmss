// Package errors provides custom error types for the PVMSS application.
// These error types enable consistent error handling, wrapping, and
// type-based error checking using errors.Is and errors.As.
package errors

import (
	"errors"
	"fmt"
)

// Standard sentinel errors for common conditions.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrValidation     = errors.New("validation failed")
	ErrInternal       = errors.New("internal error")
	ErrTimeout        = errors.New("operation timed out")
	ErrUnavailable    = errors.New("service unavailable")
	ErrConflict       = errors.New("resource conflict")
	ErrRateLimited    = errors.New("rate limited")
	ErrNotImplemented = errors.New("not implemented")
)

// ErrorCode represents a machine-readable error code.
type ErrorCode string

// Error codes for categorization.
const (
	CodeNotFound       ErrorCode = "NOT_FOUND"
	CodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	CodeForbidden      ErrorCode = "FORBIDDEN"
	CodeValidation     ErrorCode = "VALIDATION_ERROR"
	CodeInternal       ErrorCode = "INTERNAL_ERROR"
	CodeTimeout        ErrorCode = "TIMEOUT"
	CodeUnavailable    ErrorCode = "UNAVAILABLE"
	CodeConflict       ErrorCode = "CONFLICT"
	CodeRateLimited    ErrorCode = "RATE_LIMITED"
	CodeProxmox        ErrorCode = "PROXMOX_ERROR"
	CodeVM             ErrorCode = "VM_ERROR"
	CodeAuth           ErrorCode = "AUTH_ERROR"
	CodeConfig         ErrorCode = "CONFIG_ERROR"
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
)

// AppError is the base error type for all application errors.
// It provides structured error information including code, message,
// and optional wrapped error for error chaining.
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
	Details map[string]interface{}
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error for errors.Is and errors.As support.
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails adds contextual details to the error.
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	e.Details = details
	return e
}

// WithDetail adds a single detail to the error.
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// AppErr creates an AppError with the given code and message.
func AppErr(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with additional context.
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// VMError represents errors related to VM operations.
type VMError struct {
	AppError
	VMID int
	Node string
}

// VMErr creates a VM-specific error.
func VMErr(vmid int, node string, message string) *VMError {
	return &VMError{
		AppError: AppError{
			Code:    CodeVM,
			Message: message,
		},
		VMID: vmid,
		Node: node,
	}
}

// WrapVM wraps an existing error as a VM error.
func WrapVM(err error, vmid int, node string, message string) *VMError {
	return &VMError{
		AppError: AppError{
			Code:    CodeVM,
			Message: message,
			Err:     err,
		},
		VMID: vmid,
		Node: node,
	}
}

// Error implements the error interface for VMError.
func (e *VMError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: VM %d on %s: %s: %v", e.Code, e.VMID, e.Node, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: VM %d on %s: %s", e.Code, e.VMID, e.Node, e.Message)
}

// ProxmoxError represents errors from Proxmox API operations.
type ProxmoxError struct {
	AppError
	Endpoint   string
	StatusCode int
}

// ProxmoxErr creates a Proxmox-specific error.
func ProxmoxErr(endpoint string, statusCode int, message string) *ProxmoxError {
	return &ProxmoxError{
		AppError: AppError{
			Code:    CodeProxmox,
			Message: message,
		},
		Endpoint:   endpoint,
		StatusCode: statusCode,
	}
}

// WrapProxmox wraps an existing error as a Proxmox error.
func WrapProxmox(err error, endpoint string, statusCode int, message string) *ProxmoxError {
	return &ProxmoxError{
		AppError: AppError{
			Code:    CodeProxmox,
			Message: message,
			Err:     err,
		},
		Endpoint:   endpoint,
		StatusCode: statusCode,
	}
}

// Error implements the error interface for ProxmoxError.
func (e *ProxmoxError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (status %d): %s: %v", e.Code, e.Endpoint, e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s (status %d): %s", e.Code, e.Endpoint, e.StatusCode, e.Message)
}

// ValidationError represents input validation errors.
type ValidationError struct {
	AppError
	Field string
	Value interface{}
}

// ValidationErr creates a validation error.
func ValidationErr(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		AppError: AppError{
			Code:    CodeValidation,
			Message: message,
		},
		Field: field,
		Value: value,
	}
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: field '%s': %s", e.Code, e.Field, e.Message)
}

// AuthError represents authentication and authorization errors.
type AuthError struct {
	AppError
	Username string
	Action   string
}

// AuthErr creates an authentication error.
func AuthErr(username string, action string, message string) *AuthError {
	return &AuthError{
		AppError: AppError{
			Code:    CodeAuth,
			Message: message,
		},
		Username: username,
		Action:   action,
	}
}

// WrapAuth wraps an existing error as an auth error.
func WrapAuth(err error, username string, action string, message string) *AuthError {
	return &AuthError{
		AppError: AppError{
			Code:    CodeAuth,
			Message: message,
			Err:     err,
		},
		Username: username,
		Action:   action,
	}
}

// Error implements the error interface for AuthError.
func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: user '%s' action '%s': %s: %v", e.Code, e.Username, e.Action, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: user '%s' action '%s': %s", e.Code, e.Username, e.Action, e.Message)
}

// ConfigError represents configuration errors.
type ConfigError struct {
	AppError
	Key string
}

// ConfigErr creates a configuration error.
func ConfigErr(key string, message string) *ConfigError {
	return &ConfigError{
		AppError: AppError{
			Code:    CodeConfig,
			Message: message,
		},
		Key: key,
	}
}

// Error implements the error interface for ConfigError.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s: config key '%s': %s", e.Code, e.Key, e.Message)
}

// Is checks if the target error matches this error's type or wrapped error.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if the error is an unauthorized error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsValidation checks if the error is a validation error.
func IsValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// IsVMError checks if the error is a VM error.
func IsVMError(err error) bool {
	var ve *VMError
	return errors.As(err, &ve)
}

// IsProxmoxError checks if the error is a Proxmox error.
func IsProxmoxError(err error) bool {
	var pe *ProxmoxError
	return errors.As(err, &pe)
}

// IsAuthError checks if the error is an auth error.
func IsAuthError(err error) bool {
	var ae *AuthError
	return errors.As(err, &ae)
}

// GetCode extracts the error code from an AppError or returns CodeInternal.
func GetCode(err error) ErrorCode {
	var vmErr *VMError
	if errors.As(err, &vmErr) {
		return vmErr.Code
	}
	var proxmoxErr *ProxmoxError
	if errors.As(err, &proxmoxErr) {
		return proxmoxErr.Code
	}
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr.Code
	}
	var configErr *ConfigError
	if errors.As(err, &configErr) {
		return configErr.Code
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}
