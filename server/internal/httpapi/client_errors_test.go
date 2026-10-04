package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pvmss/server/internal/httpapi"
)

func TestServeClientError_AcceptsAReport(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/client-errors", strings.NewReader(`{"message":"boom","path":"/vms","stack":"Error: boom\n at x"}`))
	rec := httptest.NewRecorder()
	httpapi.ServeClientError(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestServeClientError_RejectsMalformedBody(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{bad`, `{"message":"a"} trailing`, ``} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/client-errors", strings.NewReader(body))
		rec := httptest.NewRecorder()
		httpapi.ServeClientError(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestServeClientError_ToleratesUnknownFields(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/client-errors", strings.NewReader(`{"message":"boom","futureField":{"nested":true}}`))
	rec := httptest.NewRecorder()
	httpapi.ServeClientError(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
