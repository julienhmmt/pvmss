package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

const (
	ucfOwner    = "alice@pve"
	ucfOwnerB   = "bob@pve"
	ucfID       = "dev-box"
	ucfLabel    = "Dev box"
	ucfContent  = "#cloud-config\npackages:\n  - htop\n"
	ucfStamp    = "2026-01-01T00:00:00Z"
	ucfStampTwo = "2026-01-02T00:00:00Z"
)

// openUserCloudInitFilesStore opens a fully-migrated store ready for
// user_cloudinit_files CRUD.
func openUserCloudInitFilesStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "ucf.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

func insertUserCloudInitFileRow(ctx context.Context, t *testing.T, st *store.Store, owner, id, label string) {
	t.Helper()

	row := store.UserCloudInitFile{
		Owner: owner, ID: id, Label: label, Content: ucfContent,
		CreatedAt: ucfStamp, UpdatedAt: ucfStamp,
	}
	if err := st.InsertUserCloudInitFile(ctx, row); err != nil {
		t.Fatalf("InsertUserCloudInitFile(%q, %q): %v", owner, id, err)
	}
}

func ucfInsertAndList(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	insertUserCloudInitFileRow(ctx, t, st, ucfOwner, ucfID, ucfLabel)

	files, err := st.ListUserCloudInitFiles(ctx, ucfOwner)
	if err != nil {
		t.Fatalf("ListUserCloudInitFiles: %v", err)
	}

	if len(files) != 1 || files[0].ID != ucfID || files[0].Content != ucfContent {
		t.Fatalf("list = %+v, want the one inserted row", files)
	}
}

func ucfGetAndCount(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	got, found, err := st.GetUserCloudInitFile(ctx, ucfOwner, ucfID)
	if err != nil || !found {
		t.Fatalf("GetUserCloudInitFile: found=%v err=%v", found, err)
	}

	if got.Owner != ucfOwner || got.Label != ucfLabel {
		t.Errorf("row = %+v, want owner %q label %q", got, ucfOwner, ucfLabel)
	}

	if count, err := st.CountUserCloudInitFiles(ctx, ucfOwner); err != nil || count != 1 {
		t.Errorf("count = %d, %v; want 1", count, err)
	}
}

func ucfUpdate(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.UpdateUserCloudInitFile(ctx, ucfOwner, ucfID, "Dev box v2", "#cloud-config\nruncmd:\n  - echo hi\n", ucfStampTwo); err != nil {
		t.Fatalf("UpdateUserCloudInitFile: %v", err)
	}

	got, found, err := st.GetUserCloudInitFile(ctx, ucfOwner, ucfID)
	if err != nil || !found {
		t.Fatalf("GetUserCloudInitFile after update: found=%v err=%v", found, err)
	}

	if got.Label != "Dev box v2" || got.UpdatedAt != ucfStampTwo || got.CreatedAt != ucfStamp {
		t.Errorf("updated row = %+v, want new label/stamp, kept created_at", got)
	}
}

func ucfDelete(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.DeleteUserCloudInitFile(ctx, ucfOwner, ucfID); err != nil {
		t.Fatalf("DeleteUserCloudInitFile: %v", err)
	}

	if _, found, err := st.GetUserCloudInitFile(ctx, ucfOwner, ucfID); err != nil || found {
		t.Errorf("get after delete: found=%v err=%v, want gone", found, err)
	}
}

// TestUserCloudInitFiles_CRUD covers the full lifecycle for one owner:
// insert, list, get, count, update, delete. Ordered subtests share one
// SQLite fixture.
//
//nolint:paralleltest // owns a shared SQLite fixture across ordered steps
func TestUserCloudInitFiles_CRUD(t *testing.T) {
	ctx := context.Background()
	st := openUserCloudInitFilesStore(t)

	if files, err := st.ListUserCloudInitFiles(ctx, ucfOwner); err != nil || len(files) != 0 {
		t.Fatalf("fresh list = %v, %v; want empty", files, err)
	}

	t.Run("insert and list", func(t *testing.T) { ucfInsertAndList(ctx, t, st) })
	t.Run("get and count", func(t *testing.T) { ucfGetAndCount(ctx, t, st) })
	t.Run("update", func(t *testing.T) { ucfUpdate(ctx, t, st) })
	t.Run("delete", func(t *testing.T) { ucfDelete(ctx, t, st) })
}

// TestUserCloudInitFiles_OwnerIsolation — every statement filters on owner:
// bob cannot see, update, or delete alice's file, and alice's rows never leak
// into bob's list (acceptance: "bob never sees them").
//
//nolint:paralleltest // owns a shared SQLite fixture across ordered steps
func TestUserCloudInitFiles_OwnerIsolation(t *testing.T) {
	ctx := context.Background()
	st := openUserCloudInitFilesStore(t)

	insertUserCloudInitFileRow(ctx, t, st, ucfOwner, ucfID, ucfLabel)

	if _, found, err := st.GetUserCloudInitFile(ctx, ucfOwnerB, ucfID); err != nil || found {
		t.Errorf("bob get alice's file: found=%v err=%v, want not found", found, err)
	}

	if files, err := st.ListUserCloudInitFiles(ctx, ucfOwnerB); err != nil || len(files) != 0 {
		t.Errorf("bob list = %v, %v; want empty", files, err)
	}

	if err := st.UpdateUserCloudInitFile(ctx, ucfOwnerB, ucfID, "hijack", ucfContent, ucfStampTwo); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("bob update alice's file: err=%v, want sql.ErrNoRows", err)
	}

	if err := st.DeleteUserCloudInitFile(ctx, ucfOwnerB, ucfID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("bob delete alice's file: err=%v, want sql.ErrNoRows", err)
	}

	// Same id under a different owner is a distinct row — the PK is
	// (owner, id), so no cross-owner collision.
	insertUserCloudInitFileRow(ctx, t, st, ucfOwnerB, ucfID, "Bob's box")

	if count, err := st.CountUserCloudInitFiles(ctx, ucfOwner); err != nil || count != 1 {
		t.Errorf("alice count = %d, %v; want 1 (bob's same-id row is separate)", count, err)
	}
}

// TestUserCloudInitFiles_DuplicateAndMissing — insert conflict maps to
// ErrDuplicate; update/delete on a missing row return sql.ErrNoRows.
func TestUserCloudInitFiles_DuplicateAndMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := openUserCloudInitFilesStore(t)

	insertUserCloudInitFileRow(ctx, t, st, ucfOwner, ucfID, ucfLabel)
	insertAgain := store.UserCloudInitFile{
		Owner: ucfOwner, ID: ucfID, Label: ucfLabel, Content: ucfContent,
		CreatedAt: ucfStamp, UpdatedAt: ucfStamp,
	}

	if err := st.InsertUserCloudInitFile(ctx, insertAgain); !errors.Is(err, store.ErrDuplicate) {
		t.Errorf("re-insert: err=%v, want store.ErrDuplicate", err)
	}

	if err := st.UpdateUserCloudInitFile(ctx, ucfOwner, "nonexistent", "x", ucfContent, ucfStamp); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("update missing: err=%v, want sql.ErrNoRows", err)
	}

	if err := st.DeleteUserCloudInitFile(ctx, ucfOwner, "nonexistent"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("delete missing: err=%v, want sql.ErrNoRows", err)
	}
}
