//nolint:wsl_v5 // SQLite tests group setup, mutation, reopen, and assertions
package store_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

func openCloudInitStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cloudinit.db")
	st, err := store.Open(config.Configuration{DBPath: path, LogLevel: "info", LogFormat: "json", LogOutput: "stdout"})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return st, path
}

//nolint:gocyclo,paralleltest // round trip owns shared SQLite fixture and covers each boundary
func TestCloudInitSnippet_RoundTripAndStates(t *testing.T) {
	ctx := context.Background()
	st, path := openCloudInitStore(t)
	_, found, err := st.GetCloudInitSnippet(ctx, "default", 101)
	if err != nil || found {
		t.Fatalf("fresh snippet = found %v, err %v; want absent", found, err)
	}
	if err := st.PutCloudInitSnippet(ctx, "default", 101, "local", "pvmss-101.yml", "#cloud-config\n", "alice"); err != nil {
		t.Fatalf("PutCloudInitSnippet: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	st, err = store.Open(config.Configuration{DBPath: path, LogLevel: "info", LogFormat: "json", LogOutput: "stdout"})
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() { _ = st.Close() }()

	snippet, found, err := st.GetCloudInitSnippet(ctx, "default", 101)
	if err != nil || !found {
		t.Fatalf("round trip found %v, err %v", found, err)
	}
	if snippet.Content != "#cloud-config\n" || snippet.Storage != "local" || snippet.Filename != "pvmss-101.yml" || snippet.UpdatedBy != "alice" {
		t.Fatalf("snippet = %+v", snippet)
	}
	if err := st.PutCloudInitSnippet(ctx, "default", 101, "local", "pvmss-101.yml", "", "alice"); err != nil {
		t.Fatalf("PutCloudInitSnippet clear: %v", err)
	}
	cleared, found, err := st.GetCloudInitSnippet(ctx, "default", 101)
	if err != nil || !found || cleared.Content != "" {
		t.Fatalf("cleared = %+v, found %v, err %v", cleared, found, err)
	}
}

//nolint:paralleltest // each case owns a temporary SQLite database
func TestCloudInitSnippet_CompositeKeyIsolationAndUpsert(t *testing.T) {
	st, _ := openCloudInitStore(t)
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	if err := st.PutCloudInitSnippet(ctx, "default", 101, "local", "pvmss-101.yml", "#cloud-config\na", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := st.PutCloudInitSnippet(ctx, "other", 101, "local", "pvmss-101.yml", "#cloud-config\nb", "bob"); err != nil {
		t.Fatal(err)
	}
	if err := st.PutCloudInitSnippet(ctx, "default", 101, "local", "pvmss-101.yml", "#cloud-config\nc", "alice"); err != nil {
		t.Fatal(err)
	}

	first, found, err := st.GetCloudInitSnippet(ctx, "default", 101)
	if err != nil || !found || first.Content != "#cloud-config\nc" {
		t.Fatalf("default snippet = %+v, found %v, err %v", first, found, err)
	}
	second, found, err := st.GetCloudInitSnippet(ctx, "other", 101)
	if err != nil || !found || second.Content != "#cloud-config\nb" {
		t.Fatalf("other snippet = %+v, found %v, err %v", second, found, err)
	}
}
