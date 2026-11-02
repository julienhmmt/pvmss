//nolint:wsl_v5 // SQLite scenarios keep transaction assertions adjacent
package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

const (
	clusterTestSecret      = "cluster-test-session-secret-with-32-bytes"
	testClusterTokenID     = "id"
	testClusterTokenSecret = "secret"
	testInvalidClusterURL  = "https://x.invalid"
	testPreserveClusterURL = "https://preserve.invalid"
	//nolint:gosec // G101: intentionally fake test cluster URL containing the word "secret"
	testPreserveSecretClusterURL = "https://preserve-secret.invalid/api2/json"
	testRestoreClusterURL        = "https://restore.invalid"
	testDisplayClusterURL        = "https://display.invalid"
	testUpdateClusterURL         = "https://updated.example.invalid/api2/json"
	//nolint:gosec // G101: intentionally fake Proxmox token ID fixture
	testUpdatedTokenID = "pvmss@pve!updated"
	//nolint:gosec // G101: intentionally fake Proxmox token ID fixture
	testPreserveTokenID = "pvmss@pve!preserve"
)

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
	row := store.ClusterRow{Name: "preserve-cluster", URL: testPreserveClusterURL, TokenID: testClusterTokenID, TokenSecret: testClusterTokenSecret}
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

// TestEnsureSeedClusters_SetsDisplayNames - the fake cluster seed now sets a
// human-readable DisplayName for each demo cluster so the sidebar doesn't
// show the raw internal name "default" on a fresh deployment.
//

func openProxmoxStore(t *testing.T, dbPath, url, tokenID, tokenSecret string) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:               dbPath,
		ClusterSource:        "proxmox",
		SessionSecret:        clusterTestSecret,
		ProxmoxURL:           url,
		ProxmoxAPITokenName:  tokenID,
		ProxmoxAPITokenValue: tokenSecret,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return st
}

//nolint:paralleltest // SQLite fixtures are intentionally serial
func TestEnsureEnvCluster_SeedsDefaultWhenEmpty(t *testing.T) {
	st := openProxmoxStore(t, filepath.Join(t.TempDir(), "envseed.db"),
		"https://env-seed.invalid/api2/json", "pvmss@pve!env", "env-token-secret")
	ctx := context.Background()

	rows, err := st.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != testStoreCluster {
		t.Fatalf("ListClusters = %+v, want a single row named %q", rows, testStoreCluster)
	}

	row, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if row.URL != "https://env-seed.invalid/api2/json" {
		t.Errorf("URL = %q, want %q", row.URL, "https://env-seed.invalid/api2/json")
	}
	if row.TokenSecret != "env-token-secret" {
		t.Errorf("TokenSecret = %q, want the decrypted env value", row.TokenSecret)
	}
}

//nolint:paralleltest // SQLite fixtures are intentionally serial
func TestEnsureEnvCluster_SkipsWhenClusterExists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "keep.db")
	ctx := context.Background()

	st := openProxmoxStore(t, dbPath,
		"https://first.invalid/api2/json", "pvmss@pve!first", "first-secret")
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	st = openProxmoxStore(t, dbPath,
		"https://second.invalid/api2/json", "pvmss@pve!second", "second-secret")

	row, err := st.GetCluster(ctx, "default")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if row.URL != "https://first.invalid/api2/json" || row.TokenSecret != "first-secret" {
		t.Errorf("env re-seed overwrote the existing row: URL=%q TokenSecret=%q", row.URL, row.TokenSecret)
	}
}

//nolint:paralleltest // SQLite fixtures are intentionally serial
func TestEnsureEnvCluster_SkipsWhenCredentialsMissing(t *testing.T) {
	st := openProxmoxStore(t, filepath.Join(t.TempDir(), "nocreds.db"), "", "", "")

	rows, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("ListClusters = %+v, want no seeded rows without credentials", rows)
	}
}

func TestEnsureSeedClusters_SetsDisplayNames(t *testing.T) {
	t.Parallel()

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

//nolint:paralleltest // serial: shared database fixture
func TestCreateCluster_SeedsPolicyRow(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := store.ClusterRow{
		Name:        "seeded-policy",
		URL:         "https://seeded-policy.invalid/api2/json",
		TokenID:     "tok",
		TokenSecret: "secret",
	}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	got, err := st.PolicyRow(ctx, "seeded-policy")
	if err != nil {
		t.Fatalf("PolicyRow: %v", err)
	}
	want := store.PolicyRow{
		Cluster: "seeded-policy", MaxSockets: 4, MaxCores: 8, MaxMemoryMB: 16384,
		MaxDiskPerVMGB: 500, MaxNetworkCards: 4, MaxSnapshots: 5,
		MaxVMPerUser: -1,
	}
	if got != want {
		t.Errorf("PolicyRow = %+v, want %+v", got, want)
	}

	// The fake-mode secondary cluster previously had no policy row at all.
	if _, err := st.PolicyRow(ctx, "secondary"); err != nil {
		t.Errorf("secondary cluster policy row: %v", err)
	}

	// Reactivating a soft-deleted cluster must not reset an edited policy row.
	got.MaxCores = 99
	if err := st.UpsertPolicyRow(ctx, got); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}
	if err := st.SoftDeleteCluster(ctx, "seeded-policy"); err != nil {
		t.Fatalf("SoftDeleteCluster: %v", err)
	}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster reactivation: %v", err)
	}
	preserved, err := st.PolicyRow(ctx, "seeded-policy")
	if err != nil {
		t.Fatalf("PolicyRow after reactivation: %v", err)
	}
	if preserved.MaxCores != 99 {
		t.Errorf("reactivation reset MaxCores to %d, want preserved 99", preserved.MaxCores)
	}
}

// The recovery CLI writes cluster rows whose token_secret_ciphertext comes
// from the standalone EncryptClusterToken; a server opened under the same
// session secret must read them back. This is the contract that makes CLI
// recovery usable at all.
//
//nolint:paralleltest // serial: shared database fixture
func TestEncryptClusterToken_ReadableByServer(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := store.ClusterRow{Name: "cli-cluster", URL: "https://cli.invalid/api2/json", TokenID: testClusterTokenID, TokenSecret: "placeholder"}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	ciphertext, err := store.EncryptClusterToken("cli-written-secret", clusterTestSecret)
	if err != nil {
		t.Fatalf("EncryptClusterToken: %v", err)
	}

	if _, err := st.DB().ExecContext(ctx, `UPDATE clusters SET token_secret_ciphertext = ? WHERE name = ?`, ciphertext, row.Name); err != nil {
		t.Fatalf("write cli ciphertext: %v", err)
	}

	got, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if got.TokenSecret != "cli-written-secret" {
		t.Fatalf("TokenSecret = %q, want the CLI-written plaintext", got.TokenSecret)
	}
}

// The encryption key is derived from the session secret: a store opened
// under a different secret must fail to decrypt, proving the derivation is
// actually bound to the secret and not a constant or a stored key.
//
//nolint:paralleltest // serial: shared database fixture
func TestStore_TokenEncryption_WrongSecretCannotDecrypt(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := store.ClusterRow{Name: "wrong-key-cluster", URL: "https://wk.invalid/api2/json", TokenID: testClusterTokenID, TokenSecret: "real-secret"}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	wrong, err := store.NewFromDBWithSecret(st.DB(), "a-different-session-secret-0123456789")
	if err != nil {
		t.Fatalf("NewFromDBWithSecret: %v", err)
	}

	if _, err := wrong.GetCluster(ctx, row.Name); err == nil {
		t.Fatal("GetCluster under the wrong secret decrypted successfully")
	}
}

// Misconfiguration must fail closed: a store with no encryption key refuses
// to write or read secrets rather than persisting them in the clear.
//
//nolint:paralleltest // serial: shared database fixture
func TestStore_NoEncryptionKeyFailsClosed(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := store.RunMigrations(ctx, db, store.Migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	keyless := store.NewFromDB(db)

	if _, err := store.NewFromDBWithSecret(db, "too-short"); err == nil {
		t.Fatal("NewFromDBWithSecret accepted a secret shorter than 32 bytes")
	}

	row := store.ClusterRow{Name: "keyless-cluster", URL: "https://kl.invalid/api2/json", TokenID: testClusterTokenID, TokenSecret: "real-secret"}
	if err := keyless.CreateCluster(ctx, row); err == nil {
		t.Fatal("CreateCluster without an encryption key stored a secret")
	}

	keyed, err := store.NewFromDBWithSecret(db, clusterTestSecret)
	if err != nil {
		t.Fatalf("NewFromDBWithSecret: %v", err)
	}
	if err := keyed.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster with key: %v", err)
	}

	if _, err := keyless.GetCluster(ctx, row.Name); err == nil {
		t.Fatal("GetCluster without an encryption key decrypted a stored secret")
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestStore_SetClusterTLS(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetClusterTLS(ctx, "no-such-cluster", store.ClusterTLS{}); !errors.Is(err, store.ErrInvalidClusterName) {
		t.Fatalf("SetClusterTLS on missing cluster = %v, want ErrInvalidClusterName", err)
	}

	row := store.ClusterRow{Name: "tls-cluster", URL: "https://tls.invalid/api2/json", TokenID: testClusterTokenID, TokenSecret: "s"}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	material := store.ClusterTLS{
		CAPEM:            "ca-pem",
		NodeFingerprints: "aa:bb",
		ClientCertPEM:    "cert-pem",
		ClientKeyPEM:     "client-key-pem",
	}
	if err := st.SetClusterTLS(ctx, row.Name, material); err != nil {
		t.Fatalf("SetClusterTLS: %v", err)
	}

	got, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if got.TLS != material {
		t.Fatalf("TLS = %+v, want %+v", got.TLS, material)
	}

	var keyCipher []byte
	if err := st.DB().QueryRowContext(ctx, `SELECT tls_client_key_ciphertext FROM clusters WHERE name = ?`, row.Name).Scan(&keyCipher); err != nil {
		t.Fatalf("read key ciphertext: %v", err)
	}
	if len(keyCipher) == 0 || strings.Contains(string(keyCipher), material.ClientKeyPEM) {
		t.Fatalf("client key stored empty or in plaintext: %x", keyCipher)
	}

	material.ClientKeyPEM = ""
	if err := st.SetClusterTLS(ctx, row.Name, material); err != nil {
		t.Fatalf("SetClusterTLS clear key: %v", err)
	}
	got, err = st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster after clear: %v", err)
	}
	if got.TLS.ClientKeyPEM != "" {
		t.Fatalf("ClientKeyPEM = %q, want cleared", got.TLS.ClientKeyPEM)
	}
}

// Discovery fills an empty display name but never overwrites a chosen label:
// a connection test must not stomp an admin-set name.
//
//nolint:paralleltest // serial: shared database fixture
func TestStore_SetDiscoveredClusterDisplayName(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetDiscoveredClusterDisplayName(ctx, "no-such-cluster", "x"); err != nil {
		t.Fatalf("missing cluster must be a silent no-op, got %v", err)
	}

	row := store.ClusterRow{Name: "disc-cluster", URL: "https://d.invalid/api2/json", TokenID: testClusterTokenID, TokenSecret: "s"}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	if err := st.SetDiscoveredClusterDisplayName(ctx, row.Name, "Discovered Name"); err != nil {
		t.Fatalf("SetDiscoveredClusterDisplayName: %v", err)
	}

	got, err := st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if got.DisplayName != "Discovered Name" {
		t.Fatalf("DisplayName = %q, want discovered value", got.DisplayName)
	}

	if err := st.SetDiscoveredClusterDisplayName(ctx, row.Name, "Overwrite Attempt"); err != nil {
		t.Fatalf("second discover: %v", err)
	}

	got, err = st.GetCluster(ctx, row.Name)
	if err != nil {
		t.Fatalf("GetCluster after overwrite attempt: %v", err)
	}
	if got.DisplayName != "Discovered Name" {
		t.Fatalf("DisplayName = %q, want unchanged %q", got.DisplayName, "Discovered Name")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestClusters_SoftDeleteRevokesBoundSessions(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	row := store.ClusterRow{Name: "session-cluster", URL: "https://session.invalid", TokenID: testClusterTokenID, TokenSecret: testClusterTokenSecret}
	if err := st.CreateCluster(ctx, row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	// Sessions bound to the removed cluster can only fail - every request
	// resolves the client through a registry that no longer has it - so they
	// are revoked in the same transaction. Cluster-less local sessions stay.
	bound := auth.SessionRecord{
		Hash:      []byte("bound-session-hash"),
		Identity:  auth.Identity{Username: "bob@pve", Cluster: row.Name},
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	local := auth.SessionRecord{
		Hash:      []byte("local-session-hash"),
		Identity:  auth.Identity{Username: "local-admin", IsAdmin: true},
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	for _, session := range []auth.SessionRecord{bound, local} {
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	if err := st.SoftDeleteCluster(ctx, row.Name); err != nil {
		t.Fatalf("SoftDeleteCluster: %v", err)
	}

	if _, err := st.FindSession(ctx, bound.Hash); err == nil {
		t.Fatal("bound session should be revoked with its cluster")
	}
	if _, err := st.FindSession(ctx, local.Hash); err != nil {
		t.Fatalf("cluster-less session must survive: %v", err)
	}
}

func TestSessions_DeleteAccountSessions(t *testing.T) {
	t.Parallel()

	st := openClusterStore(t)
	ctx := context.Background()
	newSession := func(hash, username, clusterName string) auth.SessionRecord {
		return auth.SessionRecord{
			Hash:      []byte(hash),
			Identity:  auth.Identity{Username: username, Cluster: clusterName},
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}
	}
	target := newSession("target-hash", "team@pve", "c1")
	otherCluster := newSession("other-cluster-hash", "team@pve", "c2")
	otherUser := newSession("other-user-hash", "bob@pve", "c1")
	for _, session := range []auth.SessionRecord{target, otherCluster, otherUser} {
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	if err := st.DeleteAccountSessions(ctx, "c1", "team@pve"); err != nil {
		t.Fatalf("DeleteAccountSessions: %v", err)
	}

	if _, err := st.FindSession(ctx, target.Hash); err == nil {
		t.Fatal("target session should be revoked")
	}
	for _, kept := range []auth.SessionRecord{otherCluster, otherUser} {
		if _, err := st.FindSession(ctx, kept.Hash); err != nil {
			t.Fatalf("session %s must survive: %v", kept.Hash, err)
		}
	}
}
