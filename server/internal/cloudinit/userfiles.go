package cloudinit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/store"
	"strings"
	"time"
)

// MaxUserCloudInitFiles is the per-user cap on stored cloud-init documents
// generous for real use, small enough that
// listing always ships full content.
const MaxUserCloudInitFiles = 20

var (
	// ErrUserFileInvalid reports a blank label or content that fails Validate.
	ErrUserFileInvalid = errors.New("invalid cloud-init file")
	// ErrUserFileDuplicate reports a slug collision within the owner's files.
	ErrUserFileDuplicate = errors.New("duplicate cloud-init file")
	// ErrUserFileNotFound reports a missing (owner, id) pair.
	ErrUserFileNotFound = errors.New("cloud-init file not found")
	// ErrUserFileLimit reports the owner already holds MaxUserCloudInitFiles.
	ErrUserFileLimit = errors.New("cloud-init file limit reached")
)

// userFileNotFoundError wraps ErrUserFileNotFound with the missing id.
func userFileNotFoundError(id string) error {
	return fmt.Errorf("%w: %q", ErrUserFileNotFound, id)
}

// UserFileStore is the narrow store surface the domain functions need — tests
// stub it, production passes *store.Store.
type UserFileStore interface {
	ListUserCloudInitFiles(ctx context.Context, owner string) ([]store.UserCloudInitFile, error)
	GetUserCloudInitFile(ctx context.Context, owner, id string) (store.UserCloudInitFile, bool, error)
	CountUserCloudInitFiles(ctx context.Context, owner string) (int, error)
	InsertUserCloudInitFile(ctx context.Context, f store.UserCloudInitFile) error
	UpdateUserCloudInitFile(ctx context.Context, owner, id, label, content, stamp string) error
	DeleteUserCloudInitFile(ctx context.Context, owner, id string) error
}

// ListUserFiles returns every file owned by owner, ordered by id.
func ListUserFiles(ctx context.Context, st UserFileStore, owner string) ([]store.UserCloudInitFile, error) {
	return st.ListUserCloudInitFiles(ctx, owner)
}

// GetUserFile returns one file by (owner, id) — ErrUserFileNotFound when the
// pair does not exist (which also covers "exists but belongs to someone
// else": the store never sees another owner's row).
func GetUserFile(ctx context.Context, st UserFileStore, owner, id string) (store.UserCloudInitFile, error) {
	f, found, err := st.GetUserCloudInitFile(ctx, owner, id)
	if err != nil {
		return store.UserCloudInitFile{}, err
	}

	if !found {
		return store.UserCloudInitFile{}, userFileNotFoundError(id)
	}

	return f, nil
}

// CreateUserFile validates label and content, derives the id from the label,
// enforces the per-user cap, and inserts. Content must be non-empty AND pass
// Validate — unlike a per-VM snippet (where empty means "none"), a stored
// file exists only to be applied.
func CreateUserFile(ctx context.Context, st UserFileStore, owner, label, content string) (store.UserCloudInitFile, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return store.UserCloudInitFile{}, fmt.Errorf("%w: label is required", ErrUserFileInvalid)
	}

	if content == "" {
		return store.UserCloudInitFile{}, fmt.Errorf("%w: content is required", ErrUserFileInvalid)
	}

	if err := Validate(content); err != nil {
		return store.UserCloudInitFile{}, fmt.Errorf("%w: %w", ErrUserFileInvalid, err)
	}

	count, err := st.CountUserCloudInitFiles(ctx, owner)
	if err != nil {
		return store.UserCloudInitFile{}, err
	}

	if count >= MaxUserCloudInitFiles {
		return store.UserCloudInitFile{}, ErrUserFileLimit
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	f := store.UserCloudInitFile{
		Owner: owner, ID: Slugify(label, "file"), Label: label, Content: content,
		CreatedAt: stamp, UpdatedAt: stamp,
	}

	if err := st.InsertUserCloudInitFile(ctx, f); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return store.UserCloudInitFile{}, ErrUserFileDuplicate
		}

		return store.UserCloudInitFile{}, err
	}

	return f, nil
}

// UpdateUserFile rewrites an existing file's label and content. Returns
// ErrUserFileNotFound when the (owner, id) pair does not exist.
func UpdateUserFile(ctx context.Context, st UserFileStore, owner, id, label, content string) (store.UserCloudInitFile, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return store.UserCloudInitFile{}, fmt.Errorf("%w: label is required", ErrUserFileInvalid)
	}

	if content == "" {
		return store.UserCloudInitFile{}, fmt.Errorf("%w: content is required", ErrUserFileInvalid)
	}

	if err := Validate(content); err != nil {
		return store.UserCloudInitFile{}, fmt.Errorf("%w: %w", ErrUserFileInvalid, err)
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.UpdateUserCloudInitFile(ctx, owner, id, label, content, stamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.UserCloudInitFile{}, userFileNotFoundError(id)
		}

		return store.UserCloudInitFile{}, err
	}

	return GetUserFile(ctx, st, owner, id)
}

// DeleteUserFile removes one file. Returns ErrUserFileNotFound when the
// (owner, id) pair does not exist.
func DeleteUserFile(ctx context.Context, st UserFileStore, owner, id string) error {
	if err := st.DeleteUserCloudInitFile(ctx, owner, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userFileNotFoundError(id)
		}

		return err
	}

	return nil
}
