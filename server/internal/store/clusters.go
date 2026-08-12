//nolint:wsl_v5 // cluster persistence keeps transaction steps adjacent
package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
)

var (
	// ErrDuplicateCluster indicates that an active cluster already uses a name.
	ErrDuplicateCluster = errors.New("duplicate cluster")
	// ErrLastActiveCluster prevents an instance from losing its final addressable cluster.
	ErrLastActiveCluster = errors.New("last active cluster")
	// ErrInvalidClusterName indicates a name outside the cluster-name grammar.
	ErrInvalidClusterName = errors.New("invalid cluster name")
)

var clusterNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// ClusterRow is the in-memory representation of one managed cluster. TokenSecret
// is decrypted only inside the server process and is never an API response field.
type ClusterRow struct {
	Name                  string
	URL                   string
	TLSInsecureSkipVerify bool
	TokenID               string
	TokenSecret           string
	OIDCEnabled           bool
	CreatedAt             time.Time
	RemovedAt             *time.Time
	LastTestStatus        *string
	LastTestAt            *time.Time
	LastTestMessage       *string
	ProxmoxVersion        string
}

// CreateCluster inserts a new cluster or reactivates a row that was soft-deleted.
// The name is immutable: reactivation and updates write every mutable column except name.
func (s *Store) CreateCluster(ctx context.Context, row ClusterRow) error {
	if err := validateClusterRow(row); err != nil {
		return err
	}
	ciphertext, err := s.encryptToken(row.TokenSecret)
	if err != nil {
		return err
	}
	createdAt := row.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cluster transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var removedAt sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT removed_at FROM clusters WHERE name = ?`, row.Name).Scan(&removedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `INSERT INTO clusters (name, url, tls_insecure_skip_verify, token_id, token_secret_ciphertext, oidc_enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, row.Name, row.URL, row.TLSInsecureSkipVerify, row.TokenID, ciphertext, row.OIDCEnabled, createdAt.UTC().Format(time.RFC3339Nano))
	case removedAt.Valid:
		_, err = tx.ExecContext(ctx, `UPDATE clusters SET url = ?, tls_insecure_skip_verify = ?, token_id = ?, token_secret_ciphertext = ?, oidc_enabled = ?, removed_at = NULL, last_test_status = NULL, last_test_at = NULL, last_test_message = NULL, proxmox_version = NULL WHERE name = ?`, row.URL, row.TLSInsecureSkipVerify, row.TokenID, ciphertext, row.OIDCEnabled, row.Name)
	default:
		return fmt.Errorf("%w: cluster %q already exists", ErrDuplicateCluster, row.Name)
	}
	if err != nil {
		return fmt.Errorf("write cluster: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cluster: %w", err)
	}
	return nil
}

// GetCluster returns an active cluster and decrypts its service-account secret.
func (s *Store) GetCluster(ctx context.Context, name string) (ClusterRow, error) {
	row := s.db.QueryRowContext(ctx, `SELECT name, url, tls_insecure_skip_verify, token_id, token_secret_ciphertext, oidc_enabled, created_at, removed_at, last_test_status, last_test_at, last_test_message, proxmox_version FROM clusters WHERE name = ? AND removed_at IS NULL`, name)
	return s.scanCluster(row)
}

// ListClusters returns every active cluster ordered by immutable logical name.
func (s *Store) ListClusters(ctx context.Context) ([]ClusterRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, url, tls_insecure_skip_verify, token_id, token_secret_ciphertext, oidc_enabled, created_at, removed_at, last_test_status, last_test_at, last_test_message, proxmox_version FROM clusters WHERE removed_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]ClusterRow, 0)
	for rows.Next() {
		row, err := s.scanCluster(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clusters: %w", err)
	}
	return result, nil
}

// UpdateCluster updates mutable connection settings without accepting a name change.
// When TokenSecret is empty the existing ciphertext is preserved untouched — no
// re-encryption churn on every call that leaves the secret unchanged.
func (s *Store) UpdateCluster(ctx context.Context, row ClusterRow) error {
	if err := validateClusterSettings(row, false); err != nil {
		return err
	}
	if _, err := s.GetCluster(ctx, row.Name); err != nil {
		return err
	}
	if row.TokenSecret != "" {
		ciphertext, err := s.encryptToken(row.TokenSecret)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, `UPDATE clusters SET url = ?, tls_insecure_skip_verify = ?, token_id = ?, token_secret_ciphertext = ? WHERE name = ? AND removed_at IS NULL`, row.URL, row.TLSInsecureSkipVerify, row.TokenID, ciphertext, row.Name)
		if err != nil {
			return fmt.Errorf("update cluster: %w", err)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE clusters SET url = ?, tls_insecure_skip_verify = ?, token_id = ? WHERE name = ? AND removed_at IS NULL`, row.URL, row.TLSInsecureSkipVerify, row.TokenID, row.Name)
	if err != nil {
		return fmt.Errorf("update cluster: %w", err)
	}
	return nil
}

// SoftDeleteCluster removes a cluster from active registries and clears its secret.
func (s *Store) SoftDeleteCluster(ctx context.Context, name string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cluster removal: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Check existence before the active-count guard so deleting a non-existent
	// cluster always surfaces as "not found" rather than a misleading
	// last-active error when zero clusters remain.
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM clusters WHERE name = ? AND removed_at IS NULL`, name).Scan(&exists); err != nil {
		return fmt.Errorf("find active cluster: %w", err)
	}
	if exists == 0 {
		return sql.ErrNoRows
	}
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM clusters WHERE removed_at IS NULL`).Scan(&active); err != nil {
		return fmt.Errorf("count active clusters: %w", err)
	}
	if active == 1 {
		return ErrLastActiveCluster
	}
	result, err := tx.ExecContext(ctx, `UPDATE clusters SET removed_at = ?, token_secret_ciphertext = NULL WHERE name = ? AND removed_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), name)
	if err != nil {
		return fmt.Errorf("soft-delete cluster: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return fmt.Errorf("count removed cluster: %w", err)
		}
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cluster removal: %w", err)
	}
	return nil
}

// SetClusterTestResult records the latest explicit connection test.
func (s *Store) SetClusterTestResult(ctx context.Context, name, status, version, message string, testedAt time.Time) error {
	var messageValue any
	if message != "" {
		messageValue = message
	}
	var versionValue any
	if version != "" {
		versionValue = version
	}
	result, err := s.db.ExecContext(ctx, `UPDATE clusters SET last_test_status = ?, last_test_at = ?, last_test_message = ?, proxmox_version = ? WHERE name = ? AND removed_at IS NULL`, status, testedAt.UTC().Format(time.RFC3339Nano), messageValue, versionValue, name)
	if err != nil {
		return fmt.Errorf("save cluster test result: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("count cluster test result: %w", err)
	} else if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

// SetClusterOIDC updates only one active cluster's OIDC preference.
func (s *Store) SetClusterOIDC(ctx context.Context, name string, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE clusters SET oidc_enabled = ? WHERE name = ? AND removed_at IS NULL`, enabled, name)
	if err != nil {
		return fmt.Errorf("update cluster oidc: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("count cluster oidc update: %w", err)
	} else if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

type clusterScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanCluster(scanner clusterScanner) (ClusterRow, error) {
	var (
		row                             ClusterRow
		ciphertext                      []byte
		tlsInsecure, oidcEnabled        bool
		createdAt, removedAt            sql.NullString
		lastStatus, lastAt, lastMessage sql.NullString
		proxmoxVersion                  sql.NullString
	)
	if err := scanner.Scan(&row.Name, &row.URL, &tlsInsecure, &row.TokenID, &ciphertext, &oidcEnabled, &createdAt, &removedAt, &lastStatus, &lastAt, &lastMessage, &proxmoxVersion); err != nil {
		return ClusterRow{}, err
	}
	row.TLSInsecureSkipVerify = tlsInsecure
	row.OIDCEnabled = oidcEnabled
	var err error
	row.TokenSecret, err = s.decryptToken(ciphertext)
	if err != nil {
		return ClusterRow{}, fmt.Errorf("decrypt cluster token: %w", err)
	}
	row.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt.String)
	if err != nil {
		return ClusterRow{}, fmt.Errorf("parse cluster creation: %w", err)
	}
	row.RemovedAt, err = parseOptionalTime(removedAt)
	if err != nil {
		return ClusterRow{}, fmt.Errorf("parse cluster removal: %w", err)
	}
	row.LastTestStatus = optionalString(lastStatus)
	row.LastTestMessage = optionalString(lastMessage)
	row.ProxmoxVersion = proxmoxVersion.String
	row.LastTestAt, err = parseOptionalTime(lastAt)
	if err != nil {
		return ClusterRow{}, fmt.Errorf("parse cluster test time: %w", err)
	}
	return row, nil
}

func validateClusterRow(row ClusterRow) error {
	return validateClusterSettings(row, true)
}

func validateClusterSettings(row ClusterRow, requireSecret bool) error {
	if !clusterNamePattern.MatchString(row.Name) {
		return ErrInvalidClusterName
	}
	if row.URL == "" || row.TokenID == "" {
		return errors.New("cluster url and token id are required")
	}
	if requireSecret && row.TokenSecret == "" {
		return errors.New("cluster token secret is required")
	}
	return nil
}

func (s *Store) encryptToken(secret string) ([]byte, error) {
	if len(s.encryptionKey) == 0 {
		return nil, errors.New("cluster token encryption key is not configured")
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create token gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate token nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, []byte(secret), nil), nil
}

func (s *Store) decryptToken(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}
	if len(s.encryptionKey) == 0 {
		return "", errors.New("cluster token encryption key is not configured")
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("create token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create token gcm: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("cluster token ciphertext is truncated")
	}
	nonce, payload := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("open token ciphertext: %w", err)
	}
	return string(plaintext), nil
}

func deriveEncryptionKey(secret string) ([]byte, error) {
	if len(secret) < 32 {
		return nil, errors.New("session secret must be at least 32 bytes")
	}
	digest := sha256.Sum256([]byte(secret))
	return digest[:], nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	return &parsed, err
}

func optionalString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func (s *Store) ensureSeedClusters(ctx context.Context) error {
	seeds := []ClusterRow{
		{Name: "default", URL: "https://pve-a.example.com:8006/api2/json", TokenID: "pvmss@pve!service", TokenSecret: "demo-default-service-secret"},     //nolint:gosec // deterministic fake credential is encrypted before persistence
		{Name: "secondary", URL: "https://pve-b.example.com:8006/api2/json", TokenID: "pvmss@pve!service", TokenSecret: "demo-secondary-service-secret"}, //nolint:gosec // deterministic fake credential is encrypted before persistence
		{Name: "offline-demo", URL: "https://pve-c.invalid:8006/api2/json", TokenID: "pvmss@pve!service", TokenSecret: "demo-offline-service-secret"},    //nolint:gosec // deterministic fake credential is encrypted before persistence
	}
	for _, row := range seeds {
		if err := s.CreateCluster(ctx, row); err != nil {
			if errors.Is(err, ErrDuplicateCluster) {
				continue
			}
			return err
		}
	}
	if err := s.SetClusterTestResult(ctx, "default", "ok", "8.2.4", "", time.Now().UTC()); err != nil {
		return err
	}
	if err := s.SetClusterTestResult(ctx, "secondary", "ok", "8.2.4", "", time.Now().UTC()); err != nil {
		return err
	}
	return s.SetClusterTestResult(ctx, "offline-demo", "unreachable", "", "connection refused", time.Now().UTC())
}
