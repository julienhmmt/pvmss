package recovery

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

var clusterNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidateClusterName checks that the operator-supplied cluster name matches
// the [a-z0-9-]+ grammar T15 requires. Returns exit-code 2's error condition.
func ValidateClusterName(name string) error {
	if !clusterNameRe.MatchString(name) {
		return fmt.Errorf("invalid cluster name %q: must match [a-z0-9-]+", name)
	}

	return nil
}

// MapCluster derives the single clusters row from environment variables (or
// flag overrides) and encrypts the token secret using T15's AES-256-GCM
// scheme (SHA-256-derived key, random nonce prepended to ciphertext).
// If no Proxmox credentials are available, the cluster row is still written
// with empty URL/token fields — only storage-node expansion is skipped.
func MapCluster(env Environ, clusterName string, flags ProxmoxCreds, sessionSecret string) (ClusterRow, ProxmoxCreds, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return ClusterRow{}, ProxmoxCreds{}, err
	}

	url := flags.URL
	if url == "" {
		url = env.Get("PROXMOX_URL")
	}

	tokenID := flags.TokenID
	if tokenID == "" {
		tokenID = env.Get("PROXMOX_API_TOKEN_NAME")
	}

	tokenSecret := flags.TokenSecret
	if tokenSecret == "" {
		tokenSecret = env.Get("PROXMOX_API_TOKEN_VALUE")
	}

	creds := ProxmoxCreds{URL: url, TokenID: tokenID, TokenSecret: tokenSecret}

	ciphertext, err := encryptToken(tokenSecret, sessionSecret)
	if err != nil {
		return ClusterRow{}, creds, fmt.Errorf("encrypt cluster token: %w", err)
	}

	row := ClusterRow{
		Name:                  clusterName,
		URL:                   url,
		TLSInsecureSkipVerify: false,
		TokenID:               tokenID,
		TokenSecretCiphertext: ciphertext,
		OIDCEnabled:           false,
		CreatedAt:             time.Now().UTC().Format(time.RFC3339Nano),
	}

	return row, creds, nil
}

// upsertCluster writes the clusters row using INSERT ... ON CONFLICT
// DO UPDATE (FR-007 idempotence). This mirrors store.CreateCluster's
// transaction logic but uses an upsert instead of insert-or-reactivate,
// so re-running the tool is safe.
func upsertCluster(ctx context.Context, v04DB *sql.DB, row ClusterRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO clusters (name, url, tls_insecure_skip_verify, token_id, token_secret_ciphertext, oidc_enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			url = excluded.url,
			tls_insecure_skip_verify = excluded.tls_insecure_skip_verify,
			token_id = excluded.token_id,
			token_secret_ciphertext = excluded.token_secret_ciphertext,
			oidc_enabled = excluded.oidc_enabled,
			removed_at = NULL`,
		row.Name, row.URL, row.TLSInsecureSkipVerify, row.TokenID,
		row.TokenSecretCiphertext, row.OIDCEnabled, row.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert cluster: %w", err)
	}

	return nil
}

// --- Token encryption (T15's scheme, reimplemented — same algorithm) ---

// deriveEncryptionKey derives a 32-byte AES key from the session secret
// via SHA-256. The session secret must be at least 32 bytes.
func deriveEncryptionKey(secret string) ([]byte, error) {
	if len(secret) < 32 {
		return nil, errors.New("session secret must be at least 32 bytes")
	}

	digest := sha256.Sum256([]byte(secret))

	return digest[:], nil
}

// encryptToken encrypts a token secret using AES-256-GCM with a random
// nonce prepended to the ciphertext. This is the exact same scheme as
// store.encryptToken — the two produce compatible ciphertexts decryptable
// by either code path.
func encryptToken(secret, sessionSecret string) ([]byte, error) {
	if secret == "" {
		return nil, nil // no token to encrypt — empty ciphertext is valid
	}

	key, err := deriveEncryptionKey(sessionSecret)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, []byte(secret), nil), nil
}
