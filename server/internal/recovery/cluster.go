package recovery

import (
	"context"
	"fmt"
	"pvmss/server/internal/store"
	"regexp"
	"time"
)

var clusterNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidateClusterName checks that the operator-supplied cluster name matches
// the [a-z0-9-]+ grammar requires. Returns exit-code 2's error condition.
func ValidateClusterName(name string) error {
	if !clusterNameRe.MatchString(name) {
		return fmt.Errorf("invalid cluster name %q: must match [a-z0-9-]+", name)
	}

	return nil
}

// MapCluster derives the single clusters row from environment variables (or
// flag overrides) and encrypts the token secret using AES-256-GCM
// scheme (SHA-256-derived key, random nonce prepended to ciphertext).
// If no Proxmox credentials are available, the cluster row is still written
// with empty URL/token fields - only storage-node expansion is skipped.
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
		CreatedAt:             time.Now().UTC().Format(time.RFC3339Nano),
	}

	return row, creds, nil
}

// upsertCluster writes the clusters row using INSERT ... ON CONFLICT
// DO UPDATE (idempotence). This mirrors store.CreateCluster's
// transaction logic but uses an upsert instead of insert-or-reactivate,
// so re-running the tool is safe.
func upsertCluster(ctx context.Context, v04DB dbtx, row ClusterRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO clusters (name, url, tls_insecure_skip_verify, token_id, token_secret_ciphertext, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			url = excluded.url,
			tls_insecure_skip_verify = excluded.tls_insecure_skip_verify,
			token_id = excluded.token_id,
			token_secret_ciphertext = excluded.token_secret_ciphertext,
			removed_at = NULL`,
		row.Name, row.URL, row.TLSInsecureSkipVerify, row.TokenID,
		row.TokenSecretCiphertext, row.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert cluster: %w", err)
	}

	return nil
}

//  - Token encryption (scheme, reimplemented - same algorithm) -

// encryptToken encrypts a token secret with the store's own scheme
// (store.EncryptClusterToken), so the server can decrypt what recovery wrote.
func encryptToken(secret, sessionSecret string) ([]byte, error) {
	if secret == "" {
		return nil, nil // no token to encrypt - empty ciphertext is valid
	}

	ciphertext, err := store.EncryptClusterToken(secret, sessionSecret)
	if err != nil {
		return nil, fmt.Errorf("encrypt cluster token: %w", err)
	}

	return ciphertext, nil
}
