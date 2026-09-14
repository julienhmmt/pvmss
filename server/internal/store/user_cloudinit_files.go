package store

import (
	"context"
	"database/sql"
)

// UserCloudInitFile is one user_cloudinit_files row — a cloud-init document a
// user owns and can reuse on any cluster (cloudinit-userdata spec D3).
type UserCloudInitFile struct {
	Owner     string
	ID        string
	Label     string
	Content   string
	CreatedAt string
	UpdatedAt string
}

// ListUserCloudInitFiles returns every file owned by owner, ordered by id —
// the /cloud-init page's data source. Content is included (16 KiB × the
// per-user cap is small enough to always ship).
func (s *Store) ListUserCloudInitFiles(ctx context.Context, owner string) ([]UserCloudInitFile, error) {
	return queryCatalog(ctx, s.db, "user cloudinit files list",
		`SELECT owner, id, label, content, created_at, updated_at FROM user_cloudinit_files WHERE owner = ? ORDER BY id`,
		[]any{owner},
		func(rows *sql.Rows) (UserCloudInitFile, error) {
			var f UserCloudInitFile
			return f, rows.Scan(&f.Owner, &f.ID, &f.Label, &f.Content, &f.CreatedAt, &f.UpdatedAt)
		},
	)
}

// GetUserCloudInitFile returns one file by (owner, id) — the owner filter is
// part of the lookup so a caller can never read another user's file.
func (s *Store) GetUserCloudInitFile(ctx context.Context, owner, id string) (UserCloudInitFile, bool, error) {
	var f UserCloudInitFile

	err := s.db.QueryRowContext(ctx,
		`SELECT owner, id, label, content, created_at, updated_at FROM user_cloudinit_files WHERE owner = ? AND id = ?`,
		owner, id).Scan(&f.Owner, &f.ID, &f.Label, &f.Content, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return UserCloudInitFile{}, false, nil
	}

	if err != nil {
		return UserCloudInitFile{}, false, err
	}

	return f, true, nil
}

// CountUserCloudInitFiles returns how many files owner currently has — the
// domain layer enforces the per-user cap against it.
func (s *Store) CountUserCloudInitFiles(ctx context.Context, owner string) (int, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_cloudinit_files WHERE owner = ?`,
		owner).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// InsertUserCloudInitFile inserts a new file row. Returns ErrDuplicate if the
// (owner, id) pair already exists (ON CONFLICT DO NOTHING guards against a
// concurrent insert between the domain layer's existence check and this
// INSERT).
func (s *Store) InsertUserCloudInitFile(ctx context.Context, f UserCloudInitFile) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO user_cloudinit_files (owner, id, label, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(owner, id) DO NOTHING`,
		[]any{f.Owner, f.ID, f.Label, f.Content, f.CreatedAt, f.UpdatedAt},
	)
}

// UpdateUserCloudInitFile updates an existing file's label and content.
// Returns sql.ErrNoRows if the (owner, id) pair does not exist.
func (s *Store) UpdateUserCloudInitFile(ctx context.Context, owner, id, label, content, updatedAt string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE user_cloudinit_files SET label = ?, content = ?, updated_at = ? WHERE owner = ? AND id = ?`,
		[]any{label, content, updatedAt, owner, id},
	)
}

// DeleteUserCloudInitFile removes a file row. Returns sql.ErrNoRows if the
// (owner, id) pair did not exist. No cascade — VMs created from the file keep
// their own per-VM copy (spec D4).
func (s *Store) DeleteUserCloudInitFile(ctx context.Context, owner, id string) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM user_cloudinit_files WHERE owner = ? AND id = ?`,
		[]any{owner, id},
	)
}
