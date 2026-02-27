package errors_test

import (
	"errors"
	"fmt"
	"testing"

	apperrors "pvmss/errors"
)

func TestAppError(t *testing.T) {
	tests := []struct {
		name     string
		err      *apperrors.AppError
		wantCode apperrors.ErrorCode
		wantMsg  string
	}{
		{
			name:     "simple error",
			err:      apperrors.AppErr(apperrors.CodeInternal, "something went wrong"),
			wantCode: apperrors.CodeInternal,
			wantMsg:  "INTERNAL_ERROR: something went wrong",
		},
		{
			name:     "validation error code",
			err:      apperrors.AppErr(apperrors.CodeValidation, "invalid input"),
			wantCode: apperrors.CodeValidation,
			wantMsg:  "VALIDATION_ERROR: invalid input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", tt.err.Code, tt.wantCode)
			}
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name        string
		originalErr error
		code        apperrors.ErrorCode
		message     string
		wantWrapped bool
	}{
		{
			name:        "wrap standard error",
			originalErr: fmt.Errorf("original error"),
			code:        apperrors.CodeInternal,
			message:     "wrapped message",
			wantWrapped: true,
		},
		{
			name:        "wrap nil error",
			originalErr: nil,
			code:        apperrors.CodeNotFound,
			message:     "not found",
			wantWrapped: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := apperrors.Wrap(tt.originalErr, tt.code, tt.message)
			if wrapped.Code != tt.code {
				t.Errorf("Code = %v, want %v", wrapped.Code, tt.code)
			}
			if tt.wantWrapped && wrapped.Unwrap() == nil {
				t.Error("Unwrap() returned nil, expected wrapped error")
			}
			if !tt.wantWrapped && wrapped.Unwrap() != nil {
				t.Error("Unwrap() returned non-nil, expected nil")
			}
		})
	}
}

func TestVMError(t *testing.T) {
	tests := []struct {
		name    string
		vmid    int
		node    string
		message string
		wantStr string
	}{
		{
			name:    "basic VM error",
			vmid:    100,
			node:    "pve1",
			message: "failed to start",
			wantStr: "VM_ERROR: VM 100 on pve1: failed to start",
		},
		{
			name:    "VM error with different node",
			vmid:    200,
			node:    "pve2",
			message: "disk full",
			wantStr: "VM_ERROR: VM 200 on pve2: disk full",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apperrors.VMErr(tt.vmid, tt.node, tt.message)
			if err.Error() != tt.wantStr {
				t.Errorf("Error() = %v, want %v", err.Error(), tt.wantStr)
			}
			if err.VMID != tt.vmid {
				t.Errorf("VMID = %v, want %v", err.VMID, tt.vmid)
			}
			if err.Node != tt.node {
				t.Errorf("Node = %v, want %v", err.Node, tt.node)
			}
		})
	}
}

func TestProxmoxError(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		statusCode int
		message    string
		wantStr    string
	}{
		{
			name:       "API error",
			endpoint:   "/api2/json/nodes",
			statusCode: 500,
			message:    "internal server error",
			wantStr:    "PROXMOX_ERROR: /api2/json/nodes (status 500): internal server error",
		},
		{
			name:       "auth error",
			endpoint:   "/api2/json/access/ticket",
			statusCode: 401,
			message:    "invalid credentials",
			wantStr:    "PROXMOX_ERROR: /api2/json/access/ticket (status 401): invalid credentials",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apperrors.ProxmoxErr(tt.endpoint, tt.statusCode, tt.message)
			if err.Error() != tt.wantStr {
				t.Errorf("Error() = %v, want %v", err.Error(), tt.wantStr)
			}
			if err.Endpoint != tt.endpoint {
				t.Errorf("Endpoint = %v, want %v", err.Endpoint, tt.endpoint)
			}
			if err.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %v, want %v", err.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   interface{}
		message string
		wantStr string
	}{
		{
			name:    "string field",
			field:   "email",
			value:   "invalid",
			message: "must be a valid email",
			wantStr: "VALIDATION_ERROR: field 'email': must be a valid email",
		},
		{
			name:    "numeric field",
			field:   "cpu_cores",
			value:   -1,
			message: "must be positive",
			wantStr: "VALIDATION_ERROR: field 'cpu_cores': must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apperrors.ValidationErr(tt.field, tt.value, tt.message)
			if err.Error() != tt.wantStr {
				t.Errorf("Error() = %v, want %v", err.Error(), tt.wantStr)
			}
			if err.Field != tt.field {
				t.Errorf("Field = %v, want %v", err.Field, tt.field)
			}
		})
	}
}

func TestAuthError(t *testing.T) {
	tests := []struct {
		name     string
		username string
		action   string
		message  string
		wantStr  string
	}{
		{
			name:     "login failure",
			username: "admin",
			action:   "login",
			message:  "invalid password",
			wantStr:  "AUTH_ERROR: user 'admin' action 'login': invalid password",
		},
		{
			name:     "permission denied",
			username: "user1",
			action:   "delete_vm",
			message:  "insufficient permissions",
			wantStr:  "AUTH_ERROR: user 'user1' action 'delete_vm': insufficient permissions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apperrors.AuthErr(tt.username, tt.action, tt.message)
			if err.Error() != tt.wantStr {
				t.Errorf("Error() = %v, want %v", err.Error(), tt.wantStr)
			}
			if err.Username != tt.username {
				t.Errorf("Username = %v, want %v", err.Username, tt.username)
			}
			if err.Action != tt.action {
				t.Errorf("Action = %v, want %v", err.Action, tt.action)
			}
		})
	}
}

func TestErrorTypeChecks(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		isVMError      bool
		isProxmoxErr   bool
		isValidation   bool
		isAuthError    bool
		isNotFound     bool
		isUnauthorized bool
	}{
		{
			name:      "VM error",
			err:       apperrors.VMErr(100, "pve1", "test"),
			isVMError: true,
		},
		{
			name:         "Proxmox error",
			err:          apperrors.ProxmoxErr("/api", 500, "test"),
			isProxmoxErr: true,
		},
		{
			name:         "Validation error",
			err:          apperrors.ValidationErr("field", "value", "test"),
			isValidation: true,
		},
		{
			name:        "Auth error",
			err:         apperrors.AuthErr("user", "action", "test"),
			isAuthError: true,
		},
		{
			name:       "Not found sentinel",
			err:        apperrors.ErrNotFound,
			isNotFound: true,
		},
		{
			name:           "Unauthorized sentinel",
			err:            apperrors.ErrUnauthorized,
			isUnauthorized: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if apperrors.IsVMError(tt.err) != tt.isVMError {
				t.Errorf("IsVMError() = %v, want %v", apperrors.IsVMError(tt.err), tt.isVMError)
			}
			if apperrors.IsProxmoxError(tt.err) != tt.isProxmoxErr {
				t.Errorf("IsProxmoxError() = %v, want %v", apperrors.IsProxmoxError(tt.err), tt.isProxmoxErr)
			}
			if apperrors.IsValidation(tt.err) != tt.isValidation {
				t.Errorf("IsValidation() = %v, want %v", apperrors.IsValidation(tt.err), tt.isValidation)
			}
			if apperrors.IsAuthError(tt.err) != tt.isAuthError {
				t.Errorf("IsAuthError() = %v, want %v", apperrors.IsAuthError(tt.err), tt.isAuthError)
			}
			if apperrors.IsNotFound(tt.err) != tt.isNotFound {
				t.Errorf("IsNotFound() = %v, want %v", apperrors.IsNotFound(tt.err), tt.isNotFound)
			}
			if apperrors.IsUnauthorized(tt.err) != tt.isUnauthorized {
				t.Errorf("IsUnauthorized() = %v, want %v", apperrors.IsUnauthorized(tt.err), tt.isUnauthorized)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := apperrors.WrapVM(originalErr, 100, "pve1", "operation failed")
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("errors.Is should return true for wrapped error")
	}
	var vmErr *apperrors.VMError
	if !errors.As(wrappedErr, &vmErr) {
		t.Error("errors.As should find VMError in chain")
	}
	if vmErr.VMID != 100 {
		t.Errorf("VMID = %v, want 100", vmErr.VMID)
	}
}

func TestWithDetails(t *testing.T) {
	err := apperrors.AppErr(apperrors.CodeInternal, "test error").
		WithDetail("key1", "value1").
		WithDetail("key2", 42)
	if err.Details["key1"] != "value1" {
		t.Errorf("Details[key1] = %v, want value1", err.Details["key1"])
	}
	if err.Details["key2"] != 42 {
		t.Errorf("Details[key2] = %v, want 42", err.Details["key2"])
	}
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode apperrors.ErrorCode
	}{
		{
			name:     "AppError",
			err:      apperrors.AppErr(apperrors.CodeValidation, "test"),
			wantCode: apperrors.CodeValidation,
		},
		{
			name:     "standard error",
			err:      fmt.Errorf("standard error"),
			wantCode: apperrors.CodeInternal,
		},
		{
			name:     "VM error",
			err:      apperrors.VMErr(100, "pve1", "test"),
			wantCode: apperrors.CodeVM,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apperrors.GetCode(tt.err); got != tt.wantCode {
				t.Errorf("GetCode() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}
