package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/nodesetup"
	"testing"
)

// TestRouter_NodeSetupScript checks the embedded node setup script is served
// publicly (no session) as plain text, for GET and HEAD.
//
//nolint:paralleltest // serial: shared router and database fixtures
func TestRouter_NodeSetupScript(t *testing.T) {
	mux := newMinimalRouter(t)

	want := nodesetup.Script()

	tests := []struct {
		method   string
		wantBody []byte
	}{
		{method: http.MethodGet, wantBody: want},
		{method: http.MethodHead, wantBody: nil},
	}

	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(context.Background(), tc.method, "/api/v1/pvmss-node-setup.sh", nil)
			mux.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
				t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", ct)
			}

			if !bytes.Equal(w.Body.Bytes(), tc.wantBody) {
				t.Fatalf("body = %d bytes, want %d bytes", w.Body.Len(), len(tc.wantBody))
			}
		})
	}
}
