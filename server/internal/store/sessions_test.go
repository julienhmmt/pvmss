package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/store"
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

// Sessions are only deleted on logout, so abandoned ones would pile up
// forever. Issuing a new session sweeps the dead ones.
//
//nolint:paralleltest // serial: shared database fixture
func TestCreateSession_PurgesExpiredSessions(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	expired := sampleSessionRecord([]byte("session-hash-expired"), "judy")
	expired.ExpiresAt = time.Now().Add(-time.Minute).UTC().Truncate(time.Second)

	tooOld := sampleSessionRecord([]byte("session-hash-too-old"), "mallory")
	tooOld.CreatedAt = time.Now().Add(-auth.MaxSessionAge - time.Minute).UTC().Truncate(time.Second)

	live := sampleSessionRecord([]byte("session-hash-live"), "niaj")

	for _, session := range []auth.SessionRecord{expired, tooOld, live} {
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession %s: %v", session.Identity.Username, err)
		}
	}

	if err := st.CreateSession(ctx, sampleSessionRecord([]byte("session-hash-new"), "olivia")); err != nil {
		t.Fatalf("CreateSession new: %v", err)
	}

	for _, hash := range [][]byte{expired.Hash, tooOld.Hash} {
		if _, err := st.FindSession(ctx, hash); !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("FindSession(%s) err = %v, want sql.ErrNoRows (purged)", hash, err)
		}
	}

	for _, hash := range [][]byte{live.Hash, []byte("session-hash-new")} {
		if _, err := st.FindSession(ctx, hash); err != nil {
			t.Errorf("FindSession(%s) = %v, want the live session kept", hash, err)
		}
	}
}

// A local session is only as alive as its account row: disabling or deleting
// the administrator must kill the session even when the revoke failed.
//
//nolint:paralleltest // serial: shared database fixture
func TestFindSession_LocalAccountGate(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	seed := func(t *testing.T, username string) int64 {
		t.Helper()

		id, err := st.CreateLocalAccount(ctx, store.LocalAccount{
			Username: username, PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			CreatedAt: now, UpdatedAt: now, CreatedBy: "setup",
		})
		if err != nil {
			t.Fatalf("CreateLocalAccount %s: %v", username, err)
		}

		return id
	}

	localSession := func(hash, username string) auth.SessionRecord {
		session := sampleSessionRecord([]byte(hash), username)
		session.Identity.Provider = auth.ProviderLocal
		session.Identity.Cluster = ""

		return session
	}

	t.Run("enabled local account resolves", func(t *testing.T) {
		seed(t, "enabled-admin")

		session := localSession("session-local-enabled", "enabled-admin")
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		if _, err := st.FindSession(ctx, session.Hash); err != nil {
			t.Fatalf("FindSession = %v, want resolved", err)
		}
	})

	t.Run("disabled local account is rejected", func(t *testing.T) {
		id := seed(t, "disabled-admin")

		session := localSession("session-local-disabled", "disabled-admin")
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		if err := st.SetLocalAccountDisabled(ctx, id, true, now); err != nil {
			t.Fatalf("SetLocalAccountDisabled: %v", err)
		}

		if _, err := st.FindSession(ctx, session.Hash); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("FindSession disabled err = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("deleted local account is rejected", func(t *testing.T) {
		id := seed(t, "deleted-admin")

		session := localSession("session-local-deleted", "deleted-admin")
		if err := st.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		if err := st.DeleteLocalAccount(ctx, id); err != nil {
			t.Fatalf("DeleteLocalAccount: %v", err)
		}

		if _, err := st.FindSession(ctx, session.Hash); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("FindSession deleted err = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("pve and bootstrap sessions skip the account lookup", func(t *testing.T) {
		for _, provider := range []auth.Provider{auth.ProviderPVE, auth.ProviderBootstrap} {
			session := sampleSessionRecord([]byte("session-"+string(provider)), "no-row-"+string(provider))
			session.Identity.Provider = provider

			if err := st.CreateSession(ctx, session); err != nil {
				t.Fatalf("CreateSession %s: %v", provider, err)
			}

			if _, err := st.FindSession(ctx, session.Hash); err != nil {
				t.Errorf("FindSession %s = %v, want resolved", provider, err)
			}
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
