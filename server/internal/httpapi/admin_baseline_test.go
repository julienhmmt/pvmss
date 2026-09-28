package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// newAdminBaselineHandler builds an AdminBaseline handler backed by the
// fake cluster and a temp store, mirroring the other admin handler fixtures.
func newAdminBaselineHandler(t *testing.T) (*httpapi.AdminBaseline, *httpapi.Auth, *store.Store) {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	st := openBaselineStore(t)

	t.Cleanup(func() { _ = st.Close() })

	// Use the zero-value Fake{} directly so publications (which write to
	// the default shared state) are visible to the handler.
	registry := &singleFakeRegistry{fake: cluster.Fake{}}

	handler := httpapi.NewAdminBaseline(authHandler, registry, st, testLogger(t))

	return handler, authHandler, st
}

// singleFakeRegistry is a minimal ClientProvider that returns the same
// Fake instance for any cluster name, so the handler can read snippets
// from the default shared state that SetFakeSnippetContent writes to.
type singleFakeRegistry struct {
	fake cluster.Fake
}

func (r *singleFakeRegistry) Client(string) (cluster.Client, error) {
	return r.fake, nil
}

func (r *singleFakeRegistry) List() []string {
	return []string{auditTestCluster}
}

type baselineDTOForTest struct {
	Generated string `json:"generated"`
	Document  *struct {
		Filename string `json:"filename"`
		Command  string `json:"command"`
		Nodes    []struct {
			Node    string `json:"node"`
			Present bool   `json:"present"`
		} `json:"nodes"`
	} `json:"document"`
}

func getBaseline(t *testing.T, handler *httpapi.AdminBaseline, cookie *http.Cookie) baselineDTOForTest {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/baseline?cluster="+auditTestCluster, nil)
	req.AddCookie(cookie)
	handler.ServeBaseline(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var dto baselineDTOForTest
	if err := json.NewDecoder(recorder.Body).Decode(&dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return dto
}

// TestAdminBaseline_ReturnsGeneratedDocument - the response carries the
// generated baseline verbatim.
//
//nolint:paralleltest // serial: shared fake cluster
func TestAdminBaseline_ReturnsGeneratedDocument(t *testing.T) {
	handler, authHandler, _ := newAdminBaselineHandler(t)

	dto := getBaseline(t, handler, adminCookie(t, authHandler))

	if !strings.Contains(dto.Generated, "qemu-guest-agent") {
		t.Errorf("generated baseline missing qemu-guest-agent: %s", dto.Generated)
	}
}

// TestAdminBaseline_ReportsDocument - the response carries the baseline
// file, the command to paste and the per-node presence, read live.
//
//nolint:paralleltest // serial: shared fake cluster
func TestAdminBaseline_ReportsDocument(t *testing.T) {
	handler, authHandler, _ := newAdminBaselineHandler(t)

	dto := getBaseline(t, handler, adminCookie(t, authHandler))

	if dto.Document == nil || !strings.HasPrefix(dto.Document.Filename, "pvmss-baseline-") || len(dto.Document.Nodes) == 0 || !dto.Document.Nodes[0].Present {
		t.Fatalf("document = %+v, want the baseline present on the demo fake", dto.Document)
	}

	if !strings.Contains(dto.Document.Command, dto.Document.Filename) {
		t.Errorf("command %q does not write %s", dto.Document.Command, dto.Document.Filename)
	}
}

// openBaselineStore opens a temp store for the admin baseline tests.
func openBaselineStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "admin-baseline.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	return st
}
