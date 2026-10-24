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

// Repeated fixture names for the local account tests.
const (
	localTestUsername = "albert"
	localTestAdmin    = "admin"
)

func newLocalAccountStore(t *testing.T) *store.Store {
	t.Helper()

	db := openTestDB(t)
	ctx := context.Background()

	if err := store.RunMigrations(ctx, db, store.Migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	return store.NewFromDB(db)
}

func seedLocalAccount(t *testing.T, st *store.Store, username string) store.LocalAccount {
	t.Helper()

	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	account := store.LocalAccount{
		Username:     username,
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    localTestAdmin,
	}

	id, err := st.CreateLocalAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CreateLocalAccount: %v", err)
	}

	account.ID = id

	return account
}

func TestLocalAccounts_CreateListLookup(t *testing.T) {
	t.Parallel()

	st := newLocalAccountStore(t)
	ctx := context.Background()

	first := seedLocalAccount(t, st, localTestUsername)
	seedLocalAccount(t, st, "beatrice")

	list, err := st.LocalAccounts(ctx)
	if err != nil {
		t.Fatalf("LocalAccounts: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("LocalAccounts len = %d, want 2", len(list))
	}

	if list[0].ID != first.ID || list[0].Username != localTestUsername || list[0].CreatedBy != localTestAdmin {
		t.Errorf("first account = %+v, want albert created by admin", list[0])
	}

	// COLLATE NOCASE: a differently-cased username must resolve the same row.
	found, err := st.LocalAccountByUsername(ctx, "ALBERT")
	if err != nil {
		t.Fatalf("LocalAccountByUsername: %v", err)
	}

	if found.ID != first.ID {
		t.Errorf("case-insensitive lookup id = %d, want %d", found.ID, first.ID)
	}

	if _, err := st.LocalAccountByID(ctx, first.ID); err != nil {
		t.Fatalf("LocalAccountByID: %v", err)
	}

	count, err := st.CountLocalAccounts(ctx)
	if err != nil {
		t.Fatalf("CountLocalAccounts: %v", err)
	}

	if count != 2 {
		t.Errorf("CountLocalAccounts = %d, want 2", count)
	}
}

func TestLocalAccounts_DuplicateUsernameIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	st := newLocalAccountStore(t)
	seedLocalAccount(t, st, localTestUsername)

	now := time.Now().UTC()
	_, err := st.CreateLocalAccount(context.Background(), store.LocalAccount{
		Username:     "Albert",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	if !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("duplicate CreateLocalAccount error = %v, want ErrDuplicate", err)
	}
}

func TestLocalAccounts_MissingRows(t *testing.T) {
	t.Parallel()

	st := newLocalAccountStore(t)
	ctx := context.Background()

	if _, err := st.LocalAccountByUsername(ctx, "nobody"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unknown username error = %v, want sql.ErrNoRows", err)
	}

	if _, err := st.LocalAccountByID(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unknown id error = %v, want sql.ErrNoRows", err)
	}

	if err := st.SetLocalAccountPassword(ctx, 999, "hash", time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("password on missing id error = %v, want sql.ErrNoRows", err)
	}

	if err := st.DeleteLocalAccount(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("delete missing id error = %v, want sql.ErrNoRows", err)
	}
}

func lockLocalAccount(t *testing.T, st *store.Store, accountID int64, now time.Time) {
	t.Helper()

	ctx := context.Background()

	locked, err := st.RecordLocalLoginFailure(ctx, accountID, now, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("RecordLocalLoginFailure: %v", err)
	}

	if !locked.IsZero() {
		t.Fatalf("locked after one failure = %v, want zero", locked)
	}

	var lockExpiry time.Time

	for range 4 {
		lockExpiry, err = st.RecordLocalLoginFailure(ctx, accountID, now, 5, 15*time.Minute)
		if err != nil {
			t.Fatalf("RecordLocalLoginFailure: %v", err)
		}
	}

	wantLock := now.Add(15 * time.Minute)
	if !lockExpiry.Equal(wantLock) {
		t.Fatalf("lock expiry = %v, want %v", lockExpiry, wantLock)
	}
}

func mustLocalAccountByID(t *testing.T, st *store.Store, accountID int64) store.LocalAccount {
	t.Helper()

	stored, err := st.LocalAccountByID(context.Background(), accountID)
	if err != nil {
		t.Fatalf("LocalAccountByID: %v", err)
	}

	return stored
}

func TestLocalAccounts_PasswordAndDisabledClearLockout(t *testing.T) {
	t.Parallel()

	st := newLocalAccountStore(t)
	ctx := context.Background()
	account := seedLocalAccount(t, st, localTestUsername)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	lockLocalAccount(t, st, account.ID, now)

	stored := mustLocalAccountByID(t, st, account.ID)
	if !stored.Locked(now) {
		t.Error("account should be locked")
	}

	if stored.Locked(now.Add(16 * time.Minute)) {
		t.Error("account should be unlocked after the window")
	}

	if err := st.SetLocalAccountPassword(ctx, account.ID, "$2a$10$newhashnewhashnewhash", now); err != nil {
		t.Fatalf("SetLocalAccountPassword: %v", err)
	}

	stored = mustLocalAccountByID(t, st, account.ID)
	if stored.FailedAttempts != 0 || !stored.LockedUntil.IsZero() {
		t.Errorf("password reset left lock state: %+v", stored)
	}

	if _, err := st.RecordLocalLoginFailure(ctx, account.ID, now, 5, 15*time.Minute); err != nil {
		t.Fatalf("RecordLocalLoginFailure: %v", err)
	}

	if err := st.SetLocalAccountDisabled(ctx, account.ID, true, now); err != nil {
		t.Fatalf("SetLocalAccountDisabled: %v", err)
	}

	stored = mustLocalAccountByID(t, st, account.ID)
	if !stored.Disabled || stored.FailedAttempts != 0 {
		t.Errorf("disable did not clear state: %+v", stored)
	}
}

func TestLocalAccounts_RecordSuccessResetsAndStampsLogin(t *testing.T) {
	t.Parallel()

	st := newLocalAccountStore(t)
	ctx := context.Background()
	account := seedLocalAccount(t, st, localTestUsername)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	if _, err := st.RecordLocalLoginFailure(ctx, account.ID, now, 5, 15*time.Minute); err != nil {
		t.Fatalf("RecordLocalLoginFailure: %v", err)
	}

	if err := st.RecordLocalLoginSuccess(ctx, account.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("RecordLocalLoginSuccess: %v", err)
	}

	stored, err := st.LocalAccountByID(ctx, account.ID)
	if err != nil {
		t.Fatalf("LocalAccountByID: %v", err)
	}

	if stored.FailedAttempts != 0 {
		t.Errorf("FailedAttempts = %d, want 0", stored.FailedAttempts)
	}

	if !stored.LastLoginAt.Equal(now.Add(time.Minute)) {
		t.Errorf("LastLoginAt = %v, want %v", stored.LastLoginAt, now.Add(time.Minute))
	}
}

func TestLocalAccounts_DeleteAndRevokeSessions(t *testing.T) {
	t.Parallel()

	st := newLocalAccountStore(t)
	ctx := context.Background()
	account := seedLocalAccount(t, st, localTestUsername)
	now := time.Now().UTC()

	for _, identity := range []struct {
		hash     []byte
		provider auth.Provider
		username string
	}{
		{[]byte("local-1"), auth.ProviderLocal, localTestUsername},
		{[]byte("local-2"), auth.ProviderLocal, localTestUsername},
		{[]byte("pve-1"), auth.ProviderPVE, localTestUsername},
	} {
		err := st.CreateSession(ctx, auth.SessionRecord{
			Hash:      identity.hash,
			Identity:  auth.Identity{Username: identity.username, Provider: identity.provider, IsAdmin: true},
			CSRFToken: "csrf",
			ExpiresAt: now.Add(time.Hour),
			CreatedAt: now,
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	if err := st.DeleteSessionsForLocalUsername(ctx, localTestUsername); err != nil {
		t.Fatalf("DeleteSessionsForLocalUsername: %v", err)
	}

	for _, hash := range [][]byte{[]byte("local-1"), []byte("local-2")} {
		if _, err := st.FindSession(ctx, hash); !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("local session %s still present: %v", hash, err)
		}
	}

	if _, err := st.FindSession(ctx, []byte("pve-1")); err != nil {
		t.Errorf("pve session should survive: %v", err)
	}

	if err := st.DeleteLocalAccount(ctx, account.ID); err != nil {
		t.Fatalf("DeleteLocalAccount: %v", err)
	}

	if _, err := st.LocalAccountByID(ctx, account.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("deleted account still present: %v", err)
	}
}
