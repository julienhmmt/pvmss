package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// LocalAccount is a DB-backed PVMSS administrator. The bootstrap
// administrator (env ADMIN_PASSWORD_HASH) is not stored here: it is the
// immutable owner, so a lost database row can never lock the instance out.
type LocalAccount struct {
	ID             int64
	Username       string
	PasswordHash   string
	Disabled       bool
	FailedAttempts int
	LockedUntil    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedBy      string
	LastLoginAt    time.Time
}

// Locked reports whether the account is inside a failed-login lockout window.
func (a LocalAccount) Locked(now time.Time) bool {
	return !a.LockedUntil.IsZero() && now.Before(a.LockedUntil)
}

// CreateLocalAccount inserts an account. Returns ErrDuplicate when the
// case-insensitive unique username already exists.
func (s *Store) CreateLocalAccount(ctx context.Context, account LocalAccount) (int64, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO local_accounts (username, password_hash, disabled, created_at, updated_at, created_by) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		account.Username, account.PasswordHash, account.Disabled,
		account.CreatedAt.UTC().Format(time.RFC3339), account.UpdatedAt.UTC().Format(time.RFC3339), account.CreatedBy)
	if err != nil {
		return 0, fmt.Errorf("insert local account: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("local account rows affected: %w", err)
	}

	if affected == 0 {
		return 0, ErrDuplicate
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("local account id: %w", err)
	}

	return id, nil
}

// LocalAccounts lists every local administrator, oldest first.
func (s *Store) LocalAccounts(ctx context.Context) ([]LocalAccount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, password_hash, disabled, failed_attempts, locked_until, created_at, updated_at, created_by, last_login_at FROM local_accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list local accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	accounts := make([]LocalAccount, 0)

	for rows.Next() {
		account, err := scanLocalAccount(rows)
		if err != nil {
			return nil, err
		}

		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local accounts: %w", err)
	}

	return accounts, nil
}

// LocalAccountByUsername looks an account up by its case-insensitive username.
func (s *Store) LocalAccountByUsername(ctx context.Context, username string) (LocalAccount, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, disabled, failed_attempts, locked_until, created_at, updated_at, created_by, last_login_at FROM local_accounts WHERE username = ?`, username)

	account, err := scanLocalAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LocalAccount{}, sql.ErrNoRows
		}

		return LocalAccount{}, err
	}

	return account, nil
}

// LocalAccountByID looks an account up by id.
func (s *Store) LocalAccountByID(ctx context.Context, id int64) (LocalAccount, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, disabled, failed_attempts, locked_until, created_at, updated_at, created_by, last_login_at FROM local_accounts WHERE id = ?`, id)

	account, err := scanLocalAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LocalAccount{}, sql.ErrNoRows
		}

		return LocalAccount{}, err
	}

	return account, nil
}

// CountLocalAccounts returns how many DB-backed administrators exist. The
// bootstrap administrator is not counted.
func (s *Store) CountLocalAccounts(ctx context.Context) (int, error) {
	var count int

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM local_accounts`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count local accounts: %w", err)
	}

	return count, nil
}

// SetLocalAccountPassword replaces the password hash and clears any lockout,
// so a password reset is also the recovery path for a locked account.
func (s *Store) SetLocalAccountPassword(ctx context.Context, id int64, passwordHash string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE local_accounts SET password_hash = ?, failed_attempts = 0, locked_until = '', updated_at = ? WHERE id = ?`,
		passwordHash, now.UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update local account password: %w", err)
	}

	return requireLocalAccount(result)
}

// SetLocalAccountDisabled enables or disables an account. Both directions
// clear the lockout so an owner can hand a clean account back.
func (s *Store) SetLocalAccountDisabled(ctx context.Context, id int64, disabled bool, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE local_accounts SET disabled = ?, failed_attempts = 0, locked_until = '', updated_at = ? WHERE id = ?`,
		disabled, now.UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update local account disabled: %w", err)
	}

	return requireLocalAccount(result)
}

// DeleteLocalAccount removes an account. Callers revoke its sessions first.
func (s *Store) DeleteLocalAccount(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM local_accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete local account: %w", err)
	}

	return requireLocalAccount(result)
}

// RecordLocalLoginFailure increments the failure counter and locks the account
// once maxFailures is reached. It returns the lock expiry (zero when the
// account is still unlocked) so the caller can log the transition.
func (s *Store) RecordLocalLoginFailure(ctx context.Context, id int64, now time.Time, maxFailures int, lockFor time.Duration) (time.Time, error) {
	lockedUntil := now.Add(lockFor).UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx, `UPDATE local_accounts SET failed_attempts = failed_attempts + 1, locked_until = CASE WHEN failed_attempts + 1 >= ? THEN ? ELSE locked_until END, updated_at = ? WHERE id = ?`,
		maxFailures, lockedUntil, now.UTC().Format(time.RFC3339), id)
	if err != nil {
		return time.Time{}, fmt.Errorf("record local login failure: %w", err)
	}

	if err := requireLocalAccount(result); err != nil {
		return time.Time{}, err
	}

	var stored string
	if err := s.db.QueryRowContext(ctx, `SELECT locked_until FROM local_accounts WHERE id = ?`, id).Scan(&stored); err != nil {
		return time.Time{}, fmt.Errorf("read local account lock: %w", err)
	}

	return parseLocalTimestamp(stored)
}

// RecordLocalLoginSuccess resets the failure counter and stamps the login.
func (s *Store) RecordLocalLoginSuccess(ctx context.Context, id int64, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE local_accounts SET failed_attempts = 0, locked_until = '', last_login_at = ?, updated_at = ? WHERE id = ?`,
		now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("record local login success: %w", err)
	}

	return requireLocalAccount(result)
}

// DeleteSessionsForLocalUsername revokes every session of a DB-backed local
// administrator (provider "local"). PVE identities with the same username are
// untouched.
func (s *Store) DeleteSessionsForLocalUsername(ctx context.Context, username string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE provider = ? AND username = ?`, "local", username)
	if err != nil {
		return fmt.Errorf("delete local account sessions: %w", err)
	}

	return nil
}

// DeleteOtherSessionsByProvider revokes every session of one identity except
// keep. It extends DeleteOtherSessions to identities whose cluster is empty
// (local administrators), which would otherwise collide with any other
// clusterless identity.
func (s *Store) DeleteOtherSessionsByProvider(ctx context.Context, provider, username string, keep []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE provider = ? AND username = ? AND token_hash != ?`, provider, username, keep)
	if err != nil {
		return fmt.Errorf("delete other sessions by provider: %w", err)
	}

	return nil
}

func requireLocalAccount(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("local account rows affected: %w", err)
	}

	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

type localAccountScanner interface {
	Scan(dest ...any) error
}

func scanLocalAccount(scanner localAccountScanner) (LocalAccount, error) {
	var (
		account              LocalAccount
		disabled             bool
		lockedUntil          string
		createdAt, updatedAt string
		lastLoginAt          string
	)

	if err := scanner.Scan(&account.ID, &account.Username, &account.PasswordHash, &disabled, &account.FailedAttempts,
		&lockedUntil, &createdAt, &updatedAt, &account.CreatedBy, &lastLoginAt); err != nil {
		return LocalAccount{}, err
	}

	account.Disabled = disabled

	var err error

	account.LockedUntil, err = parseLocalTimestamp(lockedUntil)
	if err != nil {
		return LocalAccount{}, err
	}

	account.CreatedAt, err = parseLocalTimestamp(createdAt)
	if err != nil {
		return LocalAccount{}, err
	}

	account.UpdatedAt, err = parseLocalTimestamp(updatedAt)
	if err != nil {
		return LocalAccount{}, err
	}

	account.LastLoginAt, err = parseLocalTimestamp(lastLoginAt)
	if err != nil {
		return LocalAccount{}, err
	}

	return account, nil
}

// parseLocalTimestamp accepts the empty string (zero time) and RFC3339 values.
func parseLocalTimestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse local account timestamp: %w", err)
	}

	return parsed, nil
}
