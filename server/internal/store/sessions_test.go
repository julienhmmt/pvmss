package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/auth"
	"testing"
	"time"
)

func sampleSessionRecord(hash []byte, username string) auth.SessionRecord {
	return auth.SessionRecord{
		Hash: hash,
		Identity: auth.Identity{
			Username: username,
			Pool:     "pool-" + username,
			IsAdmin:  false,
			Cluster:  testStoreCluster,
		},
		ExpiresAt: time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second),
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestCreateSession_PersistsSession(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	session := sampleSessionRecord([]byte("session-hash-create"), "grace")
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := st.FindSession(ctx, session.Hash)
	if err != nil {
		t.Fatalf("FindSession: %v", err)
	}

	if !bytesEqual(got.Hash, session.Hash) {
		t.Errorf("Hash = %v, want %v", got.Hash, session.Hash)
	}

	if got.Identity.Username != session.Identity.Username {
		t.Errorf("Username = %q, want %q", got.Identity.Username, session.Identity.Username)
	}

	if got.Identity.Pool != session.Identity.Pool {
		t.Errorf("Pool = %q, want %q", got.Identity.Pool, session.Identity.Pool)
	}

	if got.Identity.Cluster != session.Identity.Cluster {
		t.Errorf("Cluster = %q, want %q", got.Identity.Cluster, session.Identity.Cluster)
	}

	if got.Identity.IsAdmin != session.Identity.IsAdmin {
		t.Errorf("IsAdmin = %v, want %v", got.Identity.IsAdmin, session.Identity.IsAdmin)
	}

	if !got.ExpiresAt.Equal(session.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, session.ExpiresAt)
	}

	if !got.CreatedAt.Equal(session.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, session.CreatedAt)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestFindSession_NotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	_, err := st.FindSession(ctx, []byte("nonexistent-session"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindSession unknown err = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestTouchSession_UpdatesExpiry(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	session := sampleSessionRecord([]byte("session-hash-touch"), "heidi")
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	newExpiry := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := st.TouchSession(ctx, session.Hash, newExpiry); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}

	got, err := st.FindSession(ctx, session.Hash)
	if err != nil {
		t.Fatalf("FindSession: %v", err)
	}

	if !got.ExpiresAt.Equal(newExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, newExpiry)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestDeleteSession(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		session := sampleSessionRecord([]byte("session-hash-del"), "ivan")
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		if err := st.DeleteSession(ctx, session.Hash); err != nil {
			t.Fatalf("DeleteSession: %v", err)
		}

		if _, err := st.FindSession(ctx, session.Hash); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("FindSession after delete err = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("deleting absent session is not an error", func(t *testing.T) {
		if err := st.DeleteSession(ctx, []byte("never-existed")); err != nil {
			t.Fatalf("DeleteSession absent err = %v, want nil", err)
		}
	})
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
