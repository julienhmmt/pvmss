package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"time"
)

// CreateToken persists an already-hashed API token.
func (s *Store) CreateToken(ctx context.Context, token auth.TokenRecord) error {
	var expiresAt sql.NullString
	if token.ExpiresAt != nil {
		expiresAt = sql.NullString{String: token.ExpiresAt.Format(time.RFC3339), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO api_tokens (id, token_hash, username, pool, is_admin, scope, label, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, token.ID, token.Hash, token.Identity.Username, token.Identity.Pool, token.Identity.IsAdmin, token.Scope, token.Label, expiresAt, token.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert API token: %w", err)
	}

	return nil
}

// FindToken resolves a token hash without ever querying by its plaintext value.
func (s *Store) FindToken(ctx context.Context, hash []byte) (auth.TokenRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, token_hash, username, pool, is_admin, scope, label, expires_at, created_at, last_used_at FROM api_tokens WHERE token_hash = ?`, hash)
	return scanToken(row)
}

// ListTokens returns token metadata for one identity without token hashes.
func (s *Store) ListTokens(ctx context.Context, username string) ([]auth.TokenRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, token_hash, username, pool, is_admin, scope, label, expires_at, created_at, last_used_at FROM api_tokens WHERE username = ? ORDER BY created_at DESC`, username)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]auth.TokenRecord, 0)

	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, err
		}

		token.Hash = nil
		result = append(result, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate API tokens: %w", err)
	}

	return result, nil
}

// DeleteToken revokes a token owned by the supplied identity.
func (s *Store) DeleteToken(ctx context.Context, id, username string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ? AND username = ?`, id, username)
	if err != nil {
		return fmt.Errorf("delete API token: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted API tokens: %w", err)
	}

	if count == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// TouchToken records an API token's latest successful use.
func (s *Store) TouchToken(ctx context.Context, id string, usedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, usedAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update API token use: %w", err)
	}

	return nil
}

type tokenScanner interface {
	Scan(dest ...any) error
}

func scanToken(scanner tokenScanner) (auth.TokenRecord, error) {
	var (
		token      auth.TokenRecord
		isAdmin    bool
		expiresAt  sql.NullString
		createdAt  string
		lastUsedAt sql.NullString
	)
	if err := scanner.Scan(&token.ID, &token.Hash, &token.Identity.Username, &token.Identity.Pool, &isAdmin, &token.Scope, &token.Label, &expiresAt, &createdAt, &lastUsedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.TokenRecord{}, sql.ErrNoRows
		}

		return auth.TokenRecord{}, fmt.Errorf("scan API token: %w", err)
	}

	token.Identity.IsAdmin = isAdmin

	if expiresAt.Valid {
		parsed, err := time.Parse(time.RFC3339, expiresAt.String)
		if err != nil {
			return auth.TokenRecord{}, fmt.Errorf("parse token expiry: %w", err)
		}

		token.ExpiresAt = &parsed
	}

	var err error

	token.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return auth.TokenRecord{}, fmt.Errorf("parse token creation: %w", err)
	}

	if lastUsedAt.Valid {
		usedAt, err := time.Parse(time.RFC3339, lastUsedAt.String)
		if err != nil {
			return auth.TokenRecord{}, fmt.Errorf("parse token use: %w", err)
		}

		token.LastUsedAt = &usedAt
	}

	return token, nil
}
