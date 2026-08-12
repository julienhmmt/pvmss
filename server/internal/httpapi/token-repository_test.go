package httpapi_test

import (
	"context"
	"database/sql"
	"pvmss/server/internal/auth"
	"sync"
	"time"
)

type tokenRepository struct {
	mutex  sync.Mutex
	tokens map[string]auth.TokenRecord
}

func newTokenRepository() *tokenRepository {
	return &tokenRepository{tokens: make(map[string]auth.TokenRecord)}
}

func (r *tokenRepository) CreateToken(_ context.Context, token auth.TokenRecord) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.tokens[token.ID] = token

	return nil
}

func (r *tokenRepository) FindToken(_ context.Context, hash []byte) (auth.TokenRecord, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, token := range r.tokens {
		if string(token.Hash) == string(hash) {
			return token, nil
		}
	}

	return auth.TokenRecord{}, sql.ErrNoRows
}

func (r *tokenRepository) ListTokens(_ context.Context, username string) ([]auth.TokenRecord, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	result := make([]auth.TokenRecord, 0)

	for _, token := range r.tokens {
		if token.Identity.Username == username {
			result = append(result, token)
		}
	}

	return result, nil
}

func (r *tokenRepository) DeleteToken(_ context.Context, id, username string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	token, ok := r.tokens[id]
	if !ok || token.Identity.Username != username {
		return sql.ErrNoRows
	}

	delete(r.tokens, id)

	return nil
}

func (r *tokenRepository) TouchToken(_ context.Context, id string, usedAt time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	token, ok := r.tokens[id]
	if !ok {
		return sql.ErrNoRows
	}

	token.LastUsedAt = &usedAt
	r.tokens[id] = token

	return nil
}
