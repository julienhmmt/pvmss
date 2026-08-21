//nolint:wsl_v5 // SQLite scenarios keep transaction assertions adjacent
package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

const clusterTestSecret = "cluster-test-session-secret-with-32-bytes"

func openClusterStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:        filepath.Join(t.TempDir(), "clusters.db"),
		ClusterSource: "fake",
		SessionSecret: clusterTestSecret,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return st
}

//nolint:paralleltest,gosec // migration fixtures are intentionally serial and use deterministic credentials
func TestClusters_CRUDAndReactivation(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	row := store.ClusterRow{
		Name:                  "test-cluster",
		URL:                   "https://test.example.invalid/api2/json",
		TLSInsecureSkipVerify: true,
		TokenID:               "pvmss@pve!test",
		TokenSecret:           "plain-service-secret",
	}

	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	stored, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if stored.TokenSecret != row.TokenSecret {
		t.Fatalf("TokenSecret = %q, want original secret", stored.TokenSecret)
	}

	var ciphertext []byte
	if err := st.DB().QueryRowContext(ctx, "SELECT token_secret_ciphertext FROM clusters WHERE name = ?", row.Name).Scan(&ciphertext); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if len(ciphertext) == 0 || strings.Contains(string(ciphertext), row.TokenSecret) {
		t.Fatalf("ciphertext is empty or contains plaintext: %x", ciphertext)
	}

	if err := st.CreateCluster(ctx, row); !errors.Is(err, store.ErrDuplicateCluster) {
		t.Fatalf("active duplicate error = %v, want ErrDuplicateCluster", err)
	}

	if err := st.SoftDeleteCluster(ctx, row.Name); err != nil {
		t.Fatalf("SoftDeleteCluster: %v", err)
	}
	if _, err := st.GetCluster(ctx, row.Name); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetCluster after soft delete = %v, want sql.ErrNoRows", err)
	}

	row.TokenSecret = "fresh-service-secret"
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("reactivate cluster: %v", err)
	}
	stored, err = st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster after reactivation: %v", err)
	}
	if stored.TokenSecret != row.TokenSecret {
		t.Fatalf("reactivated TokenSecret = %q, want %q", stored.TokenSecret, row.TokenSecret)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestClusters_SoftDeletePreservesCatalogRows(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	row := store.ClusterRow{Name: "preserve-cluster", URL: "https://preserve.invalid", TokenID: "id", TokenSecret: "secret"}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, "INSERT INTO catalog_nodes (cluster, name, enabled) VALUES (?, ?, 1)", row.Name, "node-1"); err != nil {
		t.Fatalf("insert catalog row: %v", err)
	}
	if err := st.SoftDeleteCluster(ctx, row.Name); err != nil {
		t.Fatalf("SoftDeleteCluster: %v", err)
	}

	var count int
	if err := st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?", row.Name).Scan(&count); err != nil {
		t.Fatalf("count catalog rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("catalog row count = %d, want 1", count)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestClusters_LastActiveGuard(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("seeded active clusters = %d, want at least 2", len(rows))
	}

	for _, row := range rows[:len(rows)-1] {
		if err := st.SoftDeleteCluster(ctx, row.Name); err != nil {
			t.Fatalf("SoftDeleteCluster(%q): %v", row.Name, err)
		}
	}
	if err := st.SoftDeleteCluster(ctx, rows[len(rows)-1].Name); !errors.Is(err, store.ErrLastActiveCluster) {
		t.Fatalf("last removal error = %v, want ErrLastActiveCluster", err)
	}
}

// TestEnsureSeedClusters_SetsDisplayNames — the fake cluster seed now sets a
// human-readable DisplayName for each demo cluster so the sidebar doesn't
// show the raw internal name "default" on a fresh deployment.
//
//nolint:paralleltest // migration fixtures are intentionally serial
func TestEnsureSeedClusters_SetsDisplayNames(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}

	want := map[string]string{
		testStoreCluster: "Demo Cluster Alpha",
		"secondary":      "Demo Cluster Beta",
		"offline-demo":   "Offline Demo",
	}

	for _, row := range rows {
		expected, ok := want[row.Name]
		if !ok {
			continue
		}

		if row.DisplayName != expected {
			t.Errorf("cluster %q DisplayName = %q, want %q", row.Name, row.DisplayName, expected)
		}
	}
}
