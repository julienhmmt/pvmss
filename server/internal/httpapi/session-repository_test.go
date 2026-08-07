package httpapi_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"sync"
	"time"
)

type sessionRepository struct {
	mutex    sync.Mutex
	sessions map[string]auth.SessionRecord
}

func newSessionRepository() *sessionRepository {
	return &sessionRepository{sessions: make(map[string]auth.SessionRecord)}
}

func (r *sessionRepository) CreateSession(_ context.Context, session auth.SessionRecord) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.sessions[string(session.Hash)] = session
	return nil
}

func (r *sessionRepository) FindSession(_ context.Context, hash []byte) (auth.SessionRecord, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	session, ok := r.sessions[string(hash)]
	if !ok {
		return auth.SessionRecord{}, errors.New("session not found")
	}
	return session, nil
}

func (r *sessionRepository) TouchSession(_ context.Context, hash []byte, expiresAt time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	session, ok := r.sessions[string(hash)]
	if !ok {
		return errors.New("session not found")
	}
	session.ExpiresAt = expiresAt
	r.sessions[string(hash)] = session
	return nil
}

func (r *sessionRepository) DeleteSession(_ context.Context, hash []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.sessions, string(hash))
	return nil
}
