package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

//nolint:paralleltest // migration fixtures are intentionally serial
func TestListClusters_MultipleAndEmpty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("seeded clusters = %d, want at least 3", len(rows))
	}

	for _, r := range rows {
		if r.Name == "" {
			t.Error("cluster row has empty name")
		}
		if r.URL == "" {
			t.Errorf("cluster %q has empty URL", r.Name)
		}
		if r.TokenID == "" {
			t.Errorf("cluster %q has empty TokenID", r.Name)
		}
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestListClusters_EmptyAfterSoftDeleteAllButOne(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}

	for _, r := range rows[:len(rows)-1] {
		if err := st.SoftDeleteCluster(ctx, r.Name); err != nil {
			t.Fatalf("SoftDeleteCluster(%q): %v", r.Name, err)
		}
	}

	remaining, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters after deletes: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining clusters = %d, want 1", len(remaining))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateCluster_ExistingWithNewSecret(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	original, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}

	updated := store.ClusterRow{
		Name:        "default",
		URL:         "https://updated.example.invalid/api2/json",
		TokenID:     "pvmss@pve!updated",
		TokenSecret: "new-secret-value",
	}

	if err := st.UpdateCluster(ctx, updated); err != nil {
		t.Fatalf("UpdateCluster: %v", err)
	}

	stored, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster after update: %v", err)
	}
	if stored.URL != updated.URL {
		t.Errorf("URL = %q, want %q", stored.URL, updated.URL)
	}
	if stored.TokenID != updated.TokenID {
		t.Errorf("TokenID = %q, want %q", stored.TokenID, updated.TokenID)
	}
	if stored.TokenSecret != updated.TokenSecret {
		t.Errorf("TokenSecret = %q, want %q", stored.TokenSecret, updated.TokenSecret)
	}
	if stored.CreatedAt != original.CreatedAt {
		t.Errorf("CreatedAt changed: %v -> %v", original.CreatedAt, stored.CreatedAt)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateCluster_PreserveSecretWhenEmpty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	original, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}

	updated := store.ClusterRow{
		Name:        "default",
		URL:         "https://preserve-secret.invalid/api2/json",
		TokenID:     "pvmss@pve!preserve",
		TokenSecret: "",
	}

	if err := st.UpdateCluster(ctx, updated); err != nil {
		t.Fatalf("UpdateCluster: %v", err)
	}

	stored, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster after update: %v", err)
	}
	if stored.TokenSecret != original.TokenSecret {
		t.Errorf("TokenSecret = %q, want preserved %q", stored.TokenSecret, original.TokenSecret)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateCluster_NotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	missing := store.ClusterRow{
		Name:        "nonexistent-cluster",
		URL:         "https://nonexistent.invalid",
		TokenID:     "id",
		TokenSecret: "secret",
	}

	if err := st.UpdateCluster(ctx, missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateCluster(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateCluster_InvalidName(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	invalid := store.ClusterRow{
		Name:        "INVALID NAME",
		URL:         "https://invalid.invalid",
		TokenID:     "id",
		TokenSecret: "secret",
	}

	if err := st.UpdateCluster(ctx, invalid); !errors.Is(err, store.ErrInvalidClusterName) {
		t.Fatalf("UpdateCluster(invalid name) error = %v, want ErrInvalidClusterName", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSoftDeleteCluster_NotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SoftDeleteCluster(ctx, "nonexistent-cluster"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SoftDeleteCluster(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCreateCluster_InvalidName(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	cases := []struct {
		name string
		row  store.ClusterRow
		want error
	}{
		{
			name: "uppercase name",
			row:  store.ClusterRow{Name: "InvalidName", URL: "https://x.invalid", TokenID: "id", TokenSecret: "secret"},
			want: store.ErrInvalidClusterName,
		},
		{
			name: "spaces in name",
			row:  store.ClusterRow{Name: "has spaces", URL: "https://x.invalid", TokenID: "id", TokenSecret: "secret"},
			want: store.ErrInvalidClusterName,
		},
		{
			name: "empty URL",
			row:  store.ClusterRow{Name: "valid-name", URL: "", TokenID: "id", TokenSecret: "secret"},
			want: nil,
		},
		{
			name: "empty token ID",
			row:  store.ClusterRow{Name: "valid-name-2", URL: "https://x.invalid", TokenID: "", TokenSecret: "secret"},
			want: nil,
		},
		{
			name: "empty token secret on create",
			row:  store.ClusterRow{Name: "valid-name-3", URL: "https://x.invalid", TokenID: "id", TokenSecret: ""},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := st.CreateCluster(ctx, tc.row)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("CreateCluster error = %v, want %v", err, tc.want)
				}
			} else {
				if err == nil {
					t.Fatalf("CreateCluster should return error for %s", tc.name)
				}
			}
		})
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCreateCluster_WithDisplayName(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := store.ClusterRow{
		Name:        "display-test",
		DisplayName: "My Display Name",
		URL:         "https://display.invalid",
		TokenID:     "id",
		TokenSecret: "secret",
	}

	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	stored, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if stored.DisplayName != row.DisplayName {
		t.Errorf("DisplayName = %q, want %q", stored.DisplayName, row.DisplayName)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCreateCluster_WithCreatedAt(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	customTime := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	row := store.ClusterRow{
		Name:        "custom-time",
		URL:         "https://time.invalid",
		TokenID:     "id",
		TokenSecret: "secret",
		CreatedAt:   customTime,
	}

	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	stored, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if !stored.CreatedAt.Equal(customTime) {
		t.Errorf("CreatedAt = %v, want %v", stored.CreatedAt, customTime)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetClusterTestResult_Success(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	testedAt := time.Now().UTC()
	if err := st.SetClusterTestResult(ctx, "default", "ok", "8.2.4", "all good", testedAt); err != nil {
		t.Fatalf("SetClusterTestResult: %v", err)
	}

	stored, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if stored.LastTestStatus == nil || *stored.LastTestStatus != "ok" {
		t.Errorf("LastTestStatus = %v, want \"ok\"", stored.LastTestStatus)
	}
	if stored.ProxmoxVersion != "8.2.4" {
		t.Errorf("ProxmoxVersion = %q, want \"8.2.4\"", stored.ProxmoxVersion)
	}
	if stored.LastTestMessage == nil || *stored.LastTestMessage != "all good" {
		t.Errorf("LastTestMessage = %v, want \"all good\"", stored.LastTestMessage)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetClusterTestResult_NotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetClusterTestResult(ctx, "nonexistent", "ok", "1.0", "", time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetClusterTestResult(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetClusterOIDC_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetClusterOIDC(ctx, "default", true); err != nil {
		t.Fatalf("SetClusterOIDC(true): %v", err)
	}

	stored, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if !stored.OIDCEnabled {
		t.Error("OIDCEnabled = false, want true")
	}

	if err := st.SetClusterOIDC(ctx, "default", false); err != nil {
		t.Fatalf("SetClusterOIDC(false): %v", err)
	}

	stored, err = st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster after disable: %v", err)
	}
	if stored.OIDCEnabled {
		t.Error("OIDCEnabled = true, want false")
	}

	if err := st.SetClusterOIDC(ctx, "nonexistent", true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetClusterOIDC(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetClusterDisplayName_SuccessAndClearAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetClusterDisplayName(ctx, "default", "Production Cluster"); err != nil {
		t.Fatalf("SetClusterDisplayName: %v", err)
	}

	stored, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if stored.DisplayName != "Production Cluster" {
		t.Errorf("DisplayName = %q, want \"Production Cluster\"", stored.DisplayName)
	}

	if err := st.SetClusterDisplayName(ctx, "default", ""); err != nil {
		t.Fatalf("SetClusterDisplayName(clear): %v", err)
	}

	stored, err = st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster after clear: %v", err)
	}
	if stored.DisplayName != "" {
		t.Errorf("DisplayName = %q, want empty", stored.DisplayName)
	}

	if err := st.SetClusterDisplayName(ctx, "nonexistent", "x"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetClusterDisplayName(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestGetCluster_NotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if _, err := st.GetCluster(ctx, "nonexistent-cluster"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetCluster(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSoftDeleteCluster_RestoresAndClearsTestResults(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := store.ClusterRow{
		Name: "restore-test", URL: "https://restore.invalid", TokenID: "id", TokenSecret: "secret",
	}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if err := st.SetClusterTestResult(ctx, row.Name, "ok", "8.0", "msg", time.Now().UTC()); err != nil {
		t.Fatalf("SetClusterTestResult: %v", err)
	}

	if err := st.SoftDeleteCluster(ctx, row.Name); err != nil {
		t.Fatalf("SoftDeleteCluster: %v", err)
	}

	row.TokenSecret = "reactivated-secret"
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("Reactivate: %v", err)
	}

	stored, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster after reactivation: %v", err)
	}
	if stored.LastTestStatus != nil {
		t.Errorf("LastTestStatus = %v, want nil after reactivation", stored.LastTestStatus)
	}
	if stored.ProxmoxVersion != "" {
		t.Errorf("ProxmoxVersion = %q, want empty after reactivation", stored.ProxmoxVersion)
	}
	if stored.TokenSecret != "reactivated-secret" {
		t.Errorf("TokenSecret = %q, want reactivated-secret", stored.TokenSecret)
	}
}
