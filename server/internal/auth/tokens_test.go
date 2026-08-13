package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"pvmss/server/internal/auth"
	"sync"
	"testing"
	"time"
)

// aliceUser is the fixture identity username reused across the token tests.
const aliceUser = "alice@pve"

// tokenRepository is an in-memory auth.TokenRepository for unit-testing
// TokenService. Each method can be made to fail by setting the matching *Err
// field, so the service's error-wrapping branches are reachable without a DB.
type tokenRepository struct {
	mutex  sync.Mutex
	tokens map[string]auth.TokenRecord // keyed by hex(sha256(raw))

	createErr error
	findErr   error
	listErr   error
	deleteErr error
	touchErr  error
}

func newTokenRepository() *tokenRepository {
	return &tokenRepository{tokens: make(map[string]auth.TokenRecord)}
}

func tokenKey(hash []byte) string {
	return hex.EncodeToString(hash)
}

func (r *tokenRepository) CreateToken(_ context.Context, token auth.TokenRecord) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.createErr != nil {
		return r.createErr
	}

	r.tokens[tokenKey(token.Hash)] = token

	return nil
}

func (r *tokenRepository) FindToken(_ context.Context, hash []byte) (auth.TokenRecord, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.findErr != nil {
		return auth.TokenRecord{}, r.findErr
	}

	token, ok := r.tokens[tokenKey(hash)]
	if !ok {
		return auth.TokenRecord{}, errors.New("token not found")
	}

	return token, nil
}

func (r *tokenRepository) ListTokens(_ context.Context, username string) ([]auth.TokenRecord, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]auth.TokenRecord, 0)

	for _, token := range r.tokens {
		if token.Identity.Username == username {
			out = append(out, token)
		}
	}

	return out, nil
}

func (r *tokenRepository) DeleteToken(_ context.Context, id, username string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.deleteErr != nil {
		return r.deleteErr
	}

	for key, token := range r.tokens {
		if token.ID == id && token.Identity.Username == username {
			delete(r.tokens, key)

			return nil
		}
	}

	return errors.New("token not found")
}

func (r *tokenRepository) TouchToken(_ context.Context, id string, usedAt time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.touchErr != nil {
		return r.touchErr
	}

	for key, token := range r.tokens {
		if token.ID == id {
			token.LastUsedAt = &usedAt
			r.tokens[key] = token

			return nil
		}
	}

	return errors.New("token not found")
}

// TestNewTokenService_ConstructsService — NewTokenService wires the repository
// without any validation, so a non-nil repository yields a usable service.
func TestNewTokenService_ConstructsService(t *testing.T) {
	t.Parallel()

	service := auth.NewTokenService(newTokenRepository())
	if service == nil {
		t.Fatal("NewTokenService returned nil")
	}
}

// TestTokenService_CreateRoundTrip — a token created with a valid scope and
// label resolves back to the issuing identity, and appears in the owner's list.
//
//nolint:paralleltest // serial: shared token repository fixture
func TestTokenService_CreateRoundTrip(t *testing.T) {
	service := auth.NewTokenService(newTokenRepository())
	ctx := context.Background()

	identity := auth.Identity{Username: aliceUser, Pool: "alice", IsAdmin: false, Cluster: "default"}

	record, raw, err := service.Create(ctx, identity, "laptop", "read")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if record.ID == "" {
		t.Error("record.ID is empty")
	}

	if raw == "" {
		t.Error("raw token is empty")
	}

	if record.Label != "laptop" {
		t.Errorf("record.Label = %q, want %q", record.Label, "laptop")
	}

	if record.Scope != "read" {
		t.Errorf("record.Scope = %q, want %q", record.Scope, "read")
	}

	// Resolve must return the same identity that created the token.
	resolved, err := service.Resolve(ctx, raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolved != identity {
		t.Errorf("resolved = %+v, want %+v", resolved, identity)
	}

	// List must include the created token for the owner.
	tokens, err := service.List(ctx, identity.Username)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token in list, got %d", len(tokens))
	}

	if tokens[0].ID != record.ID {
		t.Errorf("listed token ID = %q, want %q", tokens[0].ID, record.ID)
	}
}

// TestTokenService_Create_RejectsInvalidScopeAndLabel — scope must be read or
// read_write, and a blank label (after trimming) is rejected.
func TestTokenService_Create_RejectsInvalidScopeAndLabel(t *testing.T) {
	t.Parallel()

	service := auth.NewTokenService(newTokenRepository())
	ctx := context.Background()

	identity := auth.Identity{Username: aliceUser}

	cases := []struct {
		name  string
		label string
		scope string
	}{
		{"invalid scope admin", "laptop", "admin"},
		{"invalid scope empty", "laptop", ""},
		{"empty label", "   ", "read"},
		{"blank label", "", "read_write"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := service.Create(ctx, identity, tc.label, tc.scope); err == nil {
				t.Fatalf("Create(%q, %q): expected error, got nil", tc.label, tc.scope)
			}
		})
	}
}

// TestTokenService_Create_RepositoryError — a CreateToken failure from the
// repository is wrapped and surfaced by Create.
func TestTokenService_Create_RepositoryError(t *testing.T) {
	t.Parallel()

	repository := newTokenRepository()
	repository.createErr = errors.New("disk full")

	service := auth.NewTokenService(repository)

	_, _, err := service.Create(context.Background(), auth.Identity{Username: aliceUser}, "laptop", "read")
	if err == nil {
		t.Fatal("expected Create to surface repository error, got nil")
	}
}

// TestTokenService_Resolve_RejectsUnknownAndExpired — an unknown raw token and
// an expired-but-stored token both resolve to ErrUnauthenticated.
//
//nolint:paralleltest // serial: shared token repository fixture
func TestTokenService_Resolve_RejectsUnknownAndExpired(t *testing.T) {
	repository := newTokenRepository()
	service := auth.NewTokenService(repository)
	ctx := context.Background()

	// Unknown raw value — no row matches its hash.
	if _, err := service.Resolve(ctx, "pvmss_unknown_unknown"); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("Resolve unknown: got %v, want ErrUnauthenticated", err)
	}

	// Expired token: insert directly with an ExpiresAt in the past, then resolve
	// the matching raw value — the service must reject it as unauthenticated.
	raw := "pvmss_expired_expired"
	hash := sha256.Sum256([]byte(raw))
	past := time.Now().Add(-time.Hour)

	repository.tokens[tokenKey(hash[:])] = auth.TokenRecord{
		ID:        "expired",
		Hash:      hash[:],
		Identity:  auth.Identity{Username: aliceUser},
		Scope:     "read",
		Label:     "old",
		ExpiresAt: &past,
		CreatedAt: past,
	}

	if _, err := service.Resolve(ctx, raw); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("Resolve expired: got %v, want ErrUnauthenticated", err)
	}
}

// TestTokenService_Resolve_TouchError — a TouchToken failure after a successful
// lookup is wrapped and surfaced (the token was valid, but recording its use
// failed).
//
//nolint:paralleltest // serial: shared token repository fixture
func TestTokenService_Resolve_TouchError(t *testing.T) {
	repository := newTokenRepository()
	repository.touchErr = errors.New("touch failed")

	service := auth.NewTokenService(repository)
	ctx := context.Background()

	_, raw, err := service.Create(ctx, auth.Identity{Username: aliceUser}, "laptop", "read_write")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := service.Resolve(ctx, raw); err == nil {
		t.Fatal("expected Resolve to surface TouchToken error, got nil")
	}
}

// TestTokenService_List_EmptyAndPopulated — List returns an empty slice (not
// nil, not an error) for an identity with no tokens, and the full set once
// tokens are created.
//
//nolint:paralleltest // serial: shared token repository fixture
func TestTokenService_List_EmptyAndPopulated(t *testing.T) {
	service := auth.NewTokenService(newTokenRepository())
	ctx := context.Background()

	tokens, err := service.List(ctx, aliceUser)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}

	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens for unknown user, got %d", len(tokens))
	}

	identity := auth.Identity{Username: aliceUser}

	if _, _, err := service.Create(ctx, identity, "laptop", "read"); err != nil {
		t.Fatalf("Create laptop: %v", err)
	}

	if _, _, err := service.Create(ctx, identity, "ci", "read_write"); err != nil {
		t.Fatalf("Create ci: %v", err)
	}

	tokens, err = service.List(ctx, identity.Username)
	if err != nil {
		t.Fatalf("List populated: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
}

// TestTokenService_List_RepositoryError — a ListTokens failure is wrapped and
// surfaced by List.
func TestTokenService_List_RepositoryError(t *testing.T) {
	t.Parallel()

	repository := newTokenRepository()
	repository.listErr = errors.New("scan failed")

	service := auth.NewTokenService(repository)

	if _, err := service.List(context.Background(), aliceUser); err == nil {
		t.Fatal("expected List to surface repository error, got nil")
	}
}

// TestTokenService_Revoke_OwnedAndUnknown — revoking an owned token succeeds
// and removes it from the list; revoking an unknown id (or one owned by another
// user) fails the same way, so a caller cannot probe other users' token ids.
//
//nolint:paralleltest // serial: shared token repository fixture
func TestTokenService_Revoke_OwnedAndUnknown(t *testing.T) {
	service := auth.NewTokenService(newTokenRepository())
	ctx := context.Background()

	identity := auth.Identity{Username: aliceUser}

	record, _, err := service.Create(ctx, identity, "laptop", "read")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Revoking another user's id fails identically to revoking an unknown id.
	if err := service.Revoke(ctx, record.ID, "mallory@pve"); err == nil {
		t.Fatal("expected Revoke for wrong owner to fail, got nil")
	}

	if err := service.Revoke(ctx, "nonexistent", identity.Username); err == nil {
		t.Fatal("expected Revoke for unknown id to fail, got nil")
	}

	// Revoking the owned token succeeds.
	if err := service.Revoke(ctx, record.ID, identity.Username); err != nil {
		t.Fatalf("Revoke owned: %v", err)
	}

	tokens, err := service.List(ctx, identity.Username)
	if err != nil {
		t.Fatalf("List after revoke: %v", err)
	}

	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after revoke, got %d", len(tokens))
	}
}

// TestTokenService_Revoke_RepositoryError — a DeleteToken failure is surfaced
// verbatim by Revoke (no wrapping, by design — see tokens.go).
func TestTokenService_Revoke_RepositoryError(t *testing.T) {
	t.Parallel()

	repository := newTokenRepository()
	repository.deleteErr = errors.New("delete failed")

	service := auth.NewTokenService(repository)

	if err := service.Revoke(context.Background(), "any", aliceUser); err == nil {
		t.Fatal("expected Revoke to surface repository error, got nil")
	}
}
