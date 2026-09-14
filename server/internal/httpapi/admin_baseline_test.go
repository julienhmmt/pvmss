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
func newAdminBaselineHandler(t *testing.T) (*httpapi.AdminBaseline, *httpapi.Auth) {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	st := openBaselineStore(t)

	t.Cleanup(func() { _ = st.Close() })

	// Use the zero-value Fake{} directly so SetFakeSnippetContent (which
	// writes to the default shared state) is visible to the handler.
	registry := &singleFakeRegistry{fake: cluster.Fake{}}

	handler := httpapi.NewAdminBaseline(authHandler, registry, st, testLogger(t))

	return handler, authHandler
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

// TestAdminBaseline_ReturnsGeneratedDocument — the response carries the
// generated baseline document verbatim, plus the override state.
//
//nolint:paralleltest // serial: shared fake cluster
func TestAdminBaseline_ReturnsGeneratedDocument(t *testing.T) {
	handler, authHandler := newAdminBaselineHandler(t)
	cookie := adminCookie(t, authHandler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/baseline?cluster="+auditTestCluster, nil)
	req.AddCookie(cookie)
	handler.ServeBaseline(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var dto struct {
		Generated        string `json:"generated"`
		OverridePresent  bool   `json:"overridePresent"`
		OverrideFilename string `json:"overrideFilename"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if dto.Generated == "" {
		t.Error("generated baseline is empty")
	}

	if !strings.Contains(dto.Generated, "qemu-guest-agent") {
		t.Errorf("generated baseline missing qemu-guest-agent: %s", dto.Generated)
	}

	if dto.OverrideFilename != "pvmss-baseline.yml" {
		t.Errorf("override filename = %q, want pvmss-baseline.yml", dto.OverrideFilename)
	}

	if dto.OverridePresent {
		t.Error("override should be absent in the pristine fake cluster")
	}
}

// TestAdminBaseline_ReportsOverrideWhenPresent — when a cluster-wide
// pvmss-baseline.yml exists, the response reports it as present and
// carries its content.
//
//nolint:paralleltest // serial: shared fake cluster
func TestAdminBaseline_ReportsOverrideWhenPresent(t *testing.T) {
	handler, authHandler := newAdminBaselineHandler(t)
	cookie := adminCookie(t, authHandler)

	// Seed the fake cluster with an override snippet on the first node's
	// snippet storage.
	cluster.SetFakeSnippetContent(cluster.FakeNode01, cluster.FakeSnippetStorage, "pvmss-baseline.yml", "#cloud-config\npackages:\n  - htop\n")

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/baseline?cluster="+auditTestCluster, nil)
	req.AddCookie(cookie)
	handler.ServeBaseline(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var dto struct {
		Generated        string `json:"generated"`
		OverridePresent  bool   `json:"overridePresent"`
		OverrideContent  string `json:"overrideContent"`
		OverrideFilename string `json:"overrideFilename"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !dto.OverridePresent {
		t.Error("override should be reported as present")
	}

	if dto.OverrideContent == "" {
		t.Error("override content is empty")
	}

	if !strings.Contains(dto.OverrideContent, "htop") {
		t.Errorf("override content missing htop: %s", dto.OverrideContent)
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
