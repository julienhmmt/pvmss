package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/cloudinit"
	"slices"
	"time"
)

var (
	// ErrProfileSSHKeyLimit reports a create that would exceed the per-profile
	// key cap.
	ErrProfileSSHKeyLimit = errors.New("profile ssh key limit reached")
	// ErrProfileSSHKeyDuplicateLabel reports a create whose label is already
	// used inside the same (cluster, username) profile.
	ErrProfileSSHKeyDuplicateLabel = errors.New("profile ssh key label already exists")
	// ErrProfileSSHKeyDuplicate reports a create whose key blob is already
	// stored in the same profile, even under another label or comment.
	ErrProfileSSHKeyDuplicate = errors.New("profile ssh key already stored")
)

// ProfileSSHKey is one SSH public key saved in a user's profile. The scope is
// (cluster, username) - the same username on another cluster has a separate
// key set.
type ProfileSSHKey struct {
	ID        string
	Cluster   string
	Username  string
	Label     string
	PublicKey string
	CreatedAt time.Time
}

// ListProfileSSHKeys returns the profile's keys ordered by creation time then
// id. The result is an empty non-nil slice when the profile has no keys.
func (s *Store) ListProfileSSHKeys(ctx context.Context, cluster, username string) ([]ProfileSSHKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, cluster, username, label, public_key, created_at FROM profile_ssh_keys WHERE cluster = ? AND username = ? ORDER BY created_at ASC, id ASC`, cluster, username)
	if err != nil {
		return nil, fmt.Errorf("list profile ssh keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]ProfileSSHKey, 0)

	for rows.Next() {
		key, err := scanProfileSSHKey(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profile ssh keys: %w", err)
	}

	return result, nil
}

// CreateProfileSSHKey inserts a key after enforcing the per-profile cap, the
// unique label, and fingerprint-level dedupe (the same key blob pasted with a
// different comment still collides). All three checks and the insert run in
// one transaction; an ON CONFLICT fallback reclassifies a concurrent-insert
// race into the matching sentinel.
func (s *Store) CreateProfileSSHKey(ctx context.Context, key ProfileSSHKey, maxKeys int) error {
	fingerprint, err := cloudinit.SSHKeyFingerprint(key.PublicKey)
	if err != nil {
		return fmt.Errorf("fingerprint profile ssh key: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin profile ssh key transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM profile_ssh_keys WHERE cluster = ? AND username = ?`, key.Cluster, key.Username).Scan(&count); err != nil {
		return fmt.Errorf("count profile ssh keys: %w", err)
	}

	if count >= maxKeys {
		return ErrProfileSSHKeyLimit
	}

	if err := profileSSHKeyConflict(ctx, tx, key, fingerprint); err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO profile_ssh_keys (id, cluster, username, label, public_key, created_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`, key.ID, key.Cluster, key.Username, key.Label, key.PublicKey, key.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert profile ssh key: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count inserted profile ssh keys: %w", err)
	}

	if affected == 0 {
		if err := profileSSHKeyConflict(ctx, tx, key, fingerprint); err != nil {
			return err
		}

		return ErrProfileSSHKeyDuplicate
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit profile ssh key: %w", err)
	}

	return nil
}

// DeleteProfileSSHKey removes one key owned by (cluster, username). An id that
// does not exist or belongs to another scope returns sql.ErrNoRows, so a user
// cannot probe or delete someone else's keys.
func (s *Store) DeleteProfileSSHKey(ctx context.Context, cluster, username, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM profile_ssh_keys WHERE id = ? AND cluster = ? AND username = ?`, id, cluster, username)
	if err != nil {
		return fmt.Errorf("delete profile ssh key: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted profile ssh keys: %w", err)
	}

	if count == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// profileSSHKeyConflict reports which uniqueness rule a create would violate:
// label first, then key fingerprint. Nil means the row may be inserted.
func profileSSHKeyConflict(ctx context.Context, tx *sql.Tx, key ProfileSSHKey, fingerprint string) error {
	rows, err := tx.QueryContext(ctx, `SELECT label, public_key FROM profile_ssh_keys WHERE cluster = ? AND username = ?`, key.Cluster, key.Username)
	if err != nil {
		return fmt.Errorf("read profile ssh keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var labels []string

	var publicKeys []string

	for rows.Next() {
		var label, publicKey string
		if err := rows.Scan(&label, &publicKey); err != nil {
			return fmt.Errorf("scan profile ssh key: %w", err)
		}

		labels = append(labels, label)
		publicKeys = append(publicKeys, publicKey)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate profile ssh keys: %w", err)
	}

	if slices.Contains(labels, key.Label) {
		return ErrProfileSSHKeyDuplicateLabel
	}

	for _, existing := range publicKeys {
		existingFingerprint, err := cloudinit.SSHKeyFingerprint(existing)
		if err != nil {
			continue
		}

		if existingFingerprint == fingerprint {
			return ErrProfileSSHKeyDuplicate
		}
	}

	return nil
}

type profileSSHKeyScanner interface {
	Scan(dest ...any) error
}

func scanProfileSSHKey(scanner profileSSHKeyScanner) (ProfileSSHKey, error) {
	var (
		key       ProfileSSHKey
		createdAt string
	)

	if err := scanner.Scan(&key.ID, &key.Cluster, &key.Username, &key.Label, &key.PublicKey, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProfileSSHKey{}, sql.ErrNoRows
		}

		return ProfileSSHKey{}, fmt.Errorf("scan profile ssh key: %w", err)
	}

	var err error

	key.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return ProfileSSHKey{}, fmt.Errorf("parse profile ssh key creation: %w", err)
	}

	return key, nil
}
