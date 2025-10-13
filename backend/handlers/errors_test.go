package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorResponse_StandardErrors(t *testing.T) {
	tests := []struct {
		name     string
		errResp  ErrorResponse
		wantCode int
	}{
		{"Unauthorized", ErrUnauthorized, http.StatusUnauthorized},
		{"Forbidden", ErrForbidden, http.StatusForbidden},
		{"NotFound", ErrNotFound, http.StatusNotFound},
		{"BadRequest", ErrBadRequest, http.StatusBadRequest},
		{"InternalServer", ErrInternalServer, http.StatusInternalServerError},
		{"ServiceUnavailable", ErrServiceUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errResp.Code != tt.wantCode {
				t.Errorf("Expected status code %d, got %d", tt.wantCode, tt.errResp.Code)
			}
			if tt.errResp.Key == "" {
				t.Error("Error response should have an i18n key")
			}
			if tt.errResp.Message == "" {
				t.Error("Error response should have a fallback message")
			}
		})
	}
}

func TestRespondWithError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	RespondWithError(w, req, ErrUnauthorized)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestErrorHelper(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	helper := NewErrorHelper(w, req)

	if helper == nil {
		t.Fatal("Expected ErrorHelper to be created")
	}
	if helper.Writer == nil {
		t.Error("ErrorHelper should have Writer set")
	}
	if helper.Request == nil {
		t.Error("ErrorHelper should have Request set")
	}
	if helper.Localizer == nil {
		t.Error("ErrorHelper should have Localizer set")
	}
}

func TestErrorHelper_Send(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	helper := NewErrorHelper(w, req)
	helper.Send(ErrBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLocalizeErrorWithFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Test with non-existent key should return fallback
	result := LocalizeErrorWithFallback(req, "NonExistent.Key", "Fallback Message")

	if result == "" {
		t.Error("Expected fallback message to be returned")
	}
}
