package errors_test

import (
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apperrors.GetCode(tt.err); got != tt.wantCode {
				t.Errorf("GetCode() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}
