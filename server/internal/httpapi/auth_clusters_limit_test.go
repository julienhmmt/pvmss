package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Almost every page loads the pre-login cluster list, so ordinary browsing
// must neither be throttled at the login ceiling nor spend the login budget.
//
//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthClusters_BrowsingDoesNotSpendTheLoginBudget(t *testing.T) {
	mux := newCSRFRouter(t, newAuthHandler(t))

	const clientAddr = "198.51.100.20:4000"

	for i := range 30 {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/clusters", nil)
		req.RemoteAddr = clientAddr
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("cluster list call %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"username":"alice","password":"pvmss-alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientAddr
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login after browsing status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
