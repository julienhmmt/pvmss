package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenRecord is a persisted, non-secret API token descriptor. ExpiresAt is
// nil for a token that never expires (contracts/auth-tokens.md: creation
// takes no expiry input this tranche; the column exists for later use).
type TokenRecord struct {
	ID         string
	Hash       []byte
	Identity   Identity
	Scope      string
	Label      string
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// TokenRepository is the persistence required to create, list, resolve, and revoke bearer tokens.
type TokenRepository interface {
	CreateToken(ctx context.Context, token TokenRecord) error
	FindToken(ctx context.Context, hash []byte) (TokenRecord, error)
	ListTokens(ctx context.Context, username string) ([]TokenRecord, error)
	DeleteToken(ctx context.Context, id, username string) error
	TouchToken(ctx context.Context, id string, usedAt time.Time) error
}

// TokenService creates opaque bearer tokens and resolves their saved identities.
type TokenService struct {
	repository TokenRepository
}

// NewTokenService creates an API token service backed by the supplied repository.
func NewTokenService(repository TokenRepository) *TokenService {
	return &TokenService{repository: repository}
}

// Create generates a secret shown once to the caller and persists only its hash.
func (s *TokenService) Create(ctx context.Context, identity Identity, label, scope string) (TokenRecord, string, error) {
	if scope != "read" && scope != "read_write" {
		return TokenRecord{}, "", errors.New("invalid token scope")
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return TokenRecord{}, "", errors.New("token label is required")
	}

	id, err := randomHex(12)
	if err != nil {
		return TokenRecord{}, "", err
	}

	secret, err := randomHex(32)
	if err != nil {
		return TokenRecord{}, "", err
	}

	raw := "pvmss_" + id + "_" + secret
	hash := sha256.Sum256([]byte(raw))
	now := time.Now().UTC()

	token := TokenRecord{ID: id, Hash: hash[:], Identity: identity, Scope: scope, Label: label, CreatedAt: now}
	if err := s.repository.CreateToken(ctx, token); err != nil {
		return TokenRecord{}, "", fmt.Errorf("create API token: %w", err)
	}

	return token, raw, nil
}

// Resolve validates a bearer token and records its use.
func (s *TokenService) Resolve(ctx context.Context, raw string) (Identity, error) {
	hash := sha256.Sum256([]byte(raw))

	token, err := s.repository.FindToken(ctx, hash[:])
	if err != nil || (token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now())) {
		return Identity{}, ErrUnauthenticated
	}

	if err := s.repository.TouchToken(ctx, token.ID, time.Now().UTC()); err != nil {
		return Identity{}, fmt.Errorf("record API token use: %w", err)
	}

	return token.Identity, nil
}

// List returns an identity's own tokens (label, scope, dates — never the value).
func (s *TokenService) List(ctx context.Context, username string) ([]TokenRecord, error) {
	tokens, err := s.repository.ListTokens(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}

	return tokens, nil
}

// Revoke deletes a token owned by username. Deleting an unknown or
// not-owned id fails the same way, so a caller cannot probe other users'
// token ids (contracts/auth-tokens.md).
func (s *TokenService) Revoke(ctx context.Context, id, username string) error {
	return s.repository.DeleteToken(ctx, id, username)
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read token entropy: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}
