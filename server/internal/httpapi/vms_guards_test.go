package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const caseAnonymous = "anonymous"

//nolint:paralleltest // serial: shared fake authentication state
func TestVMs_ListGuardsAndBadQueries(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := aliceCookie(t, authHandler)

	cases := []struct {
		name   string
		method string
		query  string
		cookie *http.Cookie
		want   int
		code   string
	}{
		{"wrong method", http.MethodPost, "", cookie, http.StatusMethodNotAllowed, "method_not_allowed"},
		{caseAnonymous, http.MethodGet, "", nil, http.StatusUnauthorized, "unauthenticated"},
		{"page not a number", http.MethodGet, "page=abc", cookie, http.StatusBadRequest, apiCodeInvalidRequest},
		{"pageSize not a number", http.MethodGet, "pageSize=abc", cookie, http.StatusBadRequest, apiCodeInvalidRequest},
		{"pageSize too large", http.MethodGet, "pageSize=100000", cookie, http.StatusBadRequest, "page_size_too_large"},
		{"unknown sort column", http.MethodGet, "sortBy=bogus", cookie, http.StatusBadRequest, "invalid_sort_column"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), tc.method, "/api/v1/vms?"+tc.query, nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}

			if got := errorCode(t, rec.Body.Bytes()); got != tc.code {
				t.Fatalf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()

	var envelope struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}

	return envelope.Code
}
