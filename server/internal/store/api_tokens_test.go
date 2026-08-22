package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/auth"
	"testing"
	"time"
)

func sampleTokenRecord(id string, hash []byte, username string, expiresAt *time.Time) auth.TokenRecord {
	return auth.TokenRecord{
		ID:   id,
		Hash: hash,
		Identity: auth.Identity{
			Username: username,
			Pool:     "pool-" + username,
			IsAdmin:  false,
			Cluster:  testStoreCluster,
		},
		Scope:     "vm:rw",
		Label:     "ci-token-" + id,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func assertTokenFields(t *testing.T, got, want auth.TokenRecord) {
	t.Helper()

	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}

	if got.Identity.Username != want.Identity.Username {
		t.Errorf("Username = %q, want %q", got.Identity.Username, want.Identity.Username)
	}

	if got.Scope != want.Scope {
		t.Errorf("Scope = %q, want %q", got.Scope, want.Scope)
	}

	if got.Label != want.Label {
		t.Errorf("Label = %q, want %q", got.Label, want.Label)
	}
}

func assertTokenExpiry(t *testing.T, got, want *time.Time) {
	t.Helper()

	if want == nil {
		if got != nil {
			t.Errorf("ExpiresAt = %v, want nil", got)
		}

		return
	}

	if got == nil {
		t.Fatalf("ExpiresAt = nil, want %v", want)
	}

	if !got.Equal(*want) {
		t.Errorf("ExpiresAt = %v, want %v", got, want)
	}
}

func assertListTokenEntry(t *testing.T, tk auth.TokenRecord) {
	t.Helper()

	if tk.Hash != nil {
		t.Errorf("token %s hash = %v, want nil in list output", tk.ID, tk.Hash)
	}

	if tk.Identity.Username != "carol" {
		t.Errorf("username = %q, want carol", tk.Identity.Username)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestCreateToken_WithAndWithoutExpiry(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	cases := []struct {
		name       string
		token      auth.TokenRecord
		wantExpiry *time.Time
	}{
		{
			name:       "with expiry",
			token:      sampleTokenRecord("tok-exp", []byte("hash-exp"), "alice", &expires),
			wantExpiry: &expires,
		},
		{
			name:  "without expiry",
			token: sampleTokenRecord("tok-noexp", []byte("hash-noexp"), "bob", nil),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := st.CreateToken(ctx, tc.token); err != nil {
				t.Fatalf("CreateToken: %v", err)
			}

			got, err := st.FindToken(ctx, tc.token.Hash)
			if err != nil {
				t.Fatalf("FindToken: %v", err)
			}

			assertTokenFields(t, got, tc.token)
			assertTokenExpiry(t, got.ExpiresAt, tc.wantExpiry)
		})
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestFindToken_NotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	_, err := st.FindToken(ctx, []byte("nonexistent-hash"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindToken unknown hash err = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestListTokens(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	tokens := []auth.TokenRecord{
		sampleTokenRecord("tok-a", []byte("hash-a"), "carol", nil),
		sampleTokenRecord("tok-b", []byte("hash-b"), "carol", nil),
		sampleTokenRecord("tok-c", []byte("hash-c"), "dave", nil),
	}

	for _, tk := range tokens {
		if err := st.CreateToken(ctx, tk); err != nil {
			t.Fatalf("CreateToken %s: %v", tk.ID, err)
		}
	}

	t.Run("multiple tokens for carol", func(t *testing.T) {
		got, err := st.ListTokens(ctx, "carol")
		if err != nil {
			t.Fatalf("ListTokens: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("tokens = %d, want 2", len(got))
		}

		for _, tk := range got {
			assertListTokenEntry(t, tk)
		}
	})

	t.Run("empty result for unknown user", func(t *testing.T) {
		got, err := st.ListTokens(ctx, "nobody")
		if err != nil {
			t.Fatalf("ListTokens: %v", err)
		}

		if len(got) != 0 {
			t.Errorf("tokens = %d, want 0", len(got))
		}
	})
}

//nolint:paralleltest // serial: shared database fixture
func TestDeleteToken(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	tk := sampleTokenRecord("tok-del", []byte("hash-del"), "erin", nil)
	if err := st.CreateToken(ctx, tk); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if err := st.DeleteToken(ctx, tk.ID, "erin"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	if _, err := st.FindToken(ctx, tk.Hash); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindToken after delete err = %v, want sql.ErrNoRows", err)
	}

	if err := st.DeleteToken(ctx, tk.ID, "erin"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteToken absent err = %v, want sql.ErrNoRows", err)
	}

	if err := st.DeleteToken(ctx, tk.ID, "wrong-owner"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteToken wrong owner err = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestTouchToken_UpdatesLastUsedAt(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	tk := sampleTokenRecord("tok-touch", []byte("hash-touch"), "frank", nil)
	if err := st.CreateToken(ctx, tk); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	got, err := st.FindToken(ctx, tk.Hash)
	if err != nil {
		t.Fatalf("FindToken before touch: %v", err)
	}

	if got.LastUsedAt != nil {
		t.Fatalf("LastUsedAt = %v, want nil before touch", got.LastUsedAt)
	}

	usedAt := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
	if err := st.TouchToken(ctx, tk.ID, usedAt); err != nil {
		t.Fatalf("TouchToken: %v", err)
	}

	got, err = st.FindToken(ctx, tk.Hash)
	if err != nil {
		t.Fatalf("FindToken after touch: %v", err)
	}

	if got.LastUsedAt == nil {
		t.Fatalf("LastUsedAt = nil, want %v", usedAt)
	}

	if !got.LastUsedAt.Equal(usedAt) {
		t.Errorf("LastUsedAt = %v, want %v", got.LastUsedAt, usedAt)
	}
}
