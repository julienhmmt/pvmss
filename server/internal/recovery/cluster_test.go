//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"database/sql"
	"pvmss/server/internal/recovery"
	"testing"
)

// stubEnviron is a test Environ implementation.
type stubEnviron map[string]string

func (s stubEnviron) Get(key string) string { return s[key] }

// T011: cluster row from env vars, token encryption, missing-credentials case.
func TestMapCluster_FromEnvVars(t *testing.T) {
	t.Parallel()

	env := stubEnviron{ //nolint:gosec // test fixture credential
		"PROXMOX_URL":             "https://pve.example.com:8006/api2/json",
		"PROXMOX_API_TOKEN_NAME":  "pvmss@pve!service",
		"PROXMOX_API_TOKEN_VALUE": "secret-token-value-1234567890",
	}
	secret := "test-session-secret-at-least-32-bytes!!"

	row, creds, err := recovery.MapCluster(env, "default", recovery.ProxmoxCreds{}, secret)
	if err != nil {
		t.Fatalf("MapCluster: %v", err)
	}

	if row.Name != "default" {
		t.Errorf("Name = %q, want %q", row.Name, "default")
	}

	if row.URL != env["PROXMOX_URL"] {
		t.Errorf("URL = %q, want %q", row.URL, env["PROXMOX_URL"])
	}

	if row.TokenID != env["PROXMOX_API_TOKEN_NAME"] {
		t.Errorf("TokenID = %q, want %q", row.TokenID, env["PROXMOX_API_TOKEN_NAME"])
	}

	if len(row.TokenSecretCiphertext) == 0 {
		t.Fatal("TokenSecretCiphertext is empty — token was not encrypted")
	}

	if creds.TokenSecret != env["PROXMOX_API_TOKEN_VALUE"] {
		t.Errorf("creds.TokenSecret = %q, want %q", creds.TokenSecret, env["PROXMOX_API_TOKEN_VALUE"])
	}
}

func TestMapCluster_FlagOverridesEnv(t *testing.T) {
	t.Parallel()

	env := stubEnviron{
		"PROXMOX_URL":             "https://env.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "env-token",
		"PROXMOX_API_TOKEN_VALUE": "env-secret",
	}
	flags := recovery.ProxmoxCreds{
		URL:         "https://flag.example.com:8006",
		TokenID:     "flag-token",
		TokenSecret: "flag-secret",
	}
	secret := "test-session-secret-at-least-32-bytes!!"

	row, creds, err := recovery.MapCluster(env, "prod", flags, secret)
	if err != nil {
		t.Fatalf("MapCluster: %v", err)
	}

	if row.URL != flags.URL {
		t.Errorf("URL = %q, want flag override %q", row.URL, flags.URL)
	}

	if row.TokenID != flags.TokenID {
		t.Errorf("TokenID = %q, want flag override %q", row.TokenID, flags.TokenID)
	}

	if creds.TokenSecret != flags.TokenSecret {
		t.Errorf("creds.TokenSecret = %q, want flag override", creds.TokenSecret)
	}
}

func TestMapCluster_MissingCredentials_StillSucceeds(t *testing.T) {
	t.Parallel()

	env := stubEnviron{}
	secret := "test-session-secret-at-least-32-bytes!!"

	row, creds, err := recovery.MapCluster(env, "default", recovery.ProxmoxCreds{}, secret)
	if err != nil {
		t.Fatalf("MapCluster with missing creds: %v", err)
	}

	if row.URL != "" {
		t.Errorf("URL = %q, want empty", row.URL)
	}

	if row.TokenID != "" {
		t.Errorf("TokenID = %q, want empty", row.TokenID)
	}

	if len(row.TokenSecretCiphertext) != 0 {
		t.Error("TokenSecretCiphertext should be empty when no token provided")
	}

	if creds.TokenSecret != "" {
		t.Errorf("creds.TokenSecret = %q, want empty", creds.TokenSecret)
	}
}

func TestMapCluster_InvalidName(t *testing.T) {
	t.Parallel()

	env := stubEnviron{}
	secret := "test-session-secret-at-least-32-bytes!!"

	invalidNames := []string{"Default", "with spaces", "UPPER", "under_score", ""}
	for _, name := range invalidNames {
		if _, _, err := recovery.MapCluster(env, name, recovery.ProxmoxCreds{}, secret); err == nil {
			t.Errorf("expected error for cluster name %q, got nil", name)
		}
	}
}

func TestMapCluster_TokenEncryptionIsNotCleartext(t *testing.T) {
	t.Parallel()

	env := stubEnviron{
		"PROXMOX_API_TOKEN_VALUE": "my-secret-token-1234567890123456",
	}
	secret := "test-session-secret-at-least-32-bytes!!"

	row, _, err := recovery.MapCluster(env, "default", recovery.ProxmoxCreds{}, secret)
	if err != nil {
		t.Fatalf("MapCluster: %v", err)
	}

	if string(row.TokenSecretCiphertext) == env["PROXMOX_API_TOKEN_VALUE"] {
		t.Fatal("ciphertext matches plaintext — token was not encrypted")
	}
}

// T011: cluster row is upserted into the v0.4 database.
func TestUpsertCluster_WritesRow(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	row := recovery.ClusterRow{ //nolint:gosec // test fixture ciphertext
		Name:                  "default",
		URL:                   "https://pve.example.com:8006",
		TokenID:               "pvmss@pve!service",
		TokenSecretCiphertext: []byte("encrypted-blob"),
		CreatedAt:             "2026-08-12T12:00:00Z",
	}

	if err := recovery.UpsertClusterForTest(ctx, v04DB, row); err != nil {
		t.Fatalf("UpsertCluster: %v", err)
	}

	var (
		name, url, tokenID string
		ciphertext         []byte
	)

	err := v04DB.QueryRowContext(ctx,
		`SELECT name, url, token_id, token_secret_ciphertext FROM clusters WHERE name = ?`,
		"default").Scan(&name, &url, &tokenID, &ciphertext)
	if err != nil {
		t.Fatalf("query cluster: %v", err)
	}

	if url != row.URL {
		t.Errorf("url = %q, want %q", url, row.URL)
	}

	if tokenID != row.TokenID {
		t.Errorf("token_id = %q, want %q", tokenID, row.TokenID)
	}
}

func TestUpsertCluster_Idempotent(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	row := recovery.ClusterRow{ //nolint:gosec // test fixture ciphertext
		Name:                  "default",
		URL:                   "https://pve.example.com:8006",
		TokenID:               "pvmss@pve!service",
		TokenSecretCiphertext: []byte("encrypted-blob"),
		CreatedAt:             "2026-08-12T12:00:00Z",
	}

	if err := recovery.UpsertClusterForTest(ctx, v04DB, row); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	row.URL = "https://updated.example.com:8006"
	if err := recovery.UpsertClusterForTest(ctx, v04DB, row); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var url string
	if err := v04DB.QueryRowContext(ctx,
		`SELECT url FROM clusters WHERE name = ?`, "default").Scan(&url); err != nil {
		t.Fatalf("query: %v", err)
	}

	if url != "https://updated.example.com:8006" {
		t.Errorf("url = %q, want updated value", url)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM clusters WHERE name = ?`, "default"); count != 1 {
		t.Errorf("cluster row count = %d, want 1", count)
	}
}

// stubStorageResolver is a test StorageNodeResolver.
type stubStorageResolver struct {
	nodes map[string][]string
	err   error
}

func (s stubStorageResolver) StorageNodes(_ context.Context, storageName string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.nodes[storageName], nil
}

// Ensure sql.DB is referenced for the test file's imports.
var _ = (*sql.DB)(nil)
