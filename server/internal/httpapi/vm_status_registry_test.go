package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"strings"
	"testing"
)

func newRegistryStatusBatch(t *testing.T, reader cluster.VMStatusReader) (*httpapi.VMStatusBatch, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snapshot, err := cluster.Fake{}.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{"default": &index})
	authHandler := newAuthHandler(t)

	return httpapi.NewVMStatusBatch(httpapi.VMStatusBatchDeps{
		Source:       registry,
		Auth:         authHandler,
		StatusReader: reader,
		Clients:      vmCreateClientProvider{clients: map[string]cluster.Client{auditTestCluster: cluster.Fake{}}},
		Log:          testLogger(t),
	}), authHandler
}

func postStatusBatch(handler *httpapi.VMStatusBatch, method, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, "/api/v1/vms/status", strings.NewReader(body))
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_RegistrySkipsUnresolvableTargets(t *testing.T) {
	handler, authHandler := newRegistryStatusBatch(t, cluster.Fake{})
	cookie := aliceCookie(t, authHandler)

	body := `[{"cluster":"default","vmid":101},{"cluster":"","vmid":101},{"cluster":"default","vmid":0},` +
		`{"cluster":"nonexistent","vmid":101},{"cluster":"default","vmid":999999}]`

	rec := postStatusBatch(handler, http.MethodPost, body, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var results statusBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(results) != 1 || results[0].VMID != 101 || results[0].Status == "" {
		t.Fatalf("results = %+v, want only VM 101 with a live status", results)
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_RegistryGuards(t *testing.T) {
	handler, authHandler := newRegistryStatusBatch(t, cluster.Fake{})
	cookie := aliceCookie(t, authHandler)

	tooMany := "[" + strings.Repeat(`{"cluster":"default","vmid":101},`, 100) + `{"cluster":"default","vmid":101}]`

	cases := []struct {
		name   string
		method string
		body   string
		cookie *http.Cookie
		want   int
	}{
		{"wrong method", http.MethodGet, "", cookie, http.StatusMethodNotAllowed},
		{"anonymous", http.MethodPost, `[]`, nil, http.StatusUnauthorized},
		{"malformed body", http.MethodPost, badJSONBody, cookie, http.StatusBadRequest},
		{"empty targets", http.MethodPost, `[]`, cookie, http.StatusBadRequest},
		{"too many targets", http.MethodPost, tooMany, cookie, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := postStatusBatch(handler, tc.method, tc.body, tc.cookie); rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_WithoutStatusReaderIsUnavailable(t *testing.T) {
	handler, authHandler := newRegistryStatusBatch(t, nil)

	rec := postStatusBatch(handler, http.MethodPost, `[{"cluster":"default","vmid":101}]`, aliceCookie(t, authHandler))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
