package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"time"
)

// CreateSession persists an already-hashed browser session.
func (s *Store) CreateSession(ctx context.Context, session auth.SessionRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, username, pool, cluster, is_admin, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.Hash, session.Identity.Username, session.Identity.Pool, session.Identity.Cluster, session.Identity.IsAdmin, session.ExpiresAt.Format(time.RFC3339), session.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

// FindSession resolves a session hash without ever querying by its plaintext value.
func (s *Store) FindSession(ctx context.Context, hash []byte) (auth.SessionRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT token_hash, username, pool, cluster, is_admin, expires_at, created_at FROM sessions WHERE token_hash = ?`, hash)

	var (
		session              auth.SessionRecord
		isAdmin              bool
		expiresAt, createdAt string
	)
	if err := row.Scan(&session.Hash, &session.Identity.Username, &session.Identity.Pool, &session.Identity.Cluster, &isAdmin, &expiresAt, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.SessionRecord{}, sql.ErrNoRows
		}

		return auth.SessionRecord{}, fmt.Errorf("scan session: %w", err)
	}

	session.Identity.IsAdmin = isAdmin

	var err error

	session.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return auth.SessionRecord{}, fmt.Errorf("parse session expiry: %w", err)
	}

	session.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return auth.SessionRecord{}, fmt.Errorf("parse session creation: %w", err)
	}

	return session, nil
}

// TouchSession slides a session's expiry forward.
func (s *Store) TouchSession(ctx context.Context, hash []byte, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, expiresAt.Format(time.RFC3339), hash)
	if err != nil {
		return fmt.Errorf("slide session expiry: %w", err)
	}

	return nil
}

// DeleteSession revokes a session. Deleting an already-absent session is not an error.
func (s *Store) DeleteSession(ctx context.Context, hash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}
