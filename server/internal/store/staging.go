package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// stagingTTL is how long an unconfirmed import preview remains reachable.
// Long enough for an admin to read the confirmation dialog, short enough not
// to accumulate stale uploads (T14 data-model.md).
const stagingTTL = 5 * time.Minute

// StagingEntry is the in-memory record of an uploaded-but-unconfirmed import
// file: the temp file path on disk and the preview returned to the admin.
// Never persisted — same "runtime fact" shape T10's VNCTicket established
// (AC01). Lost on restart, which is acceptable since nothing has been
// written to the live database yet at that point.
type StagingEntry struct {
	TempPath  string
	Preview   ImportPreview
	expiresAt time.Time
}

// ImportStaging is the in-memory, TTL-bound map of staging tokens to
// unconfirmed import previews. It is safe for concurrent use.
type ImportStaging struct {
	mu      sync.Mutex
	entries map[string]StagingEntry
	now     func() time.Time
}

// NewImportStaging creates an empty staging map.
func NewImportStaging() *ImportStaging {
	return &ImportStaging{
		entries: make(map[string]StagingEntry),
		now:     time.Now,
	}
}

// Stage records a new staging entry and returns its token. Two concurrent
// stages always get distinct tokens (random 16-byte hex).
func (s *ImportStaging) Stage(tempPath string, preview ImportPreview) string {
	token := newToken()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Lazily purge expired entries — no background goroutine (constitution VIII).
	s.purgeExpiredLocked()

	s.entries[token] = StagingEntry{
		TempPath:  tempPath,
		Preview:   preview,
		expiresAt: s.now().Add(stagingTTL),
	}

	return token
}

// Lookup returns the staging entry for a token, or a sentinel error:
//   - ErrStagingNotFound: the token was never staged or has been removed
//   - ErrStagingExpired: the token was staged but its TTL has elapsed
//
// An expired entry is purged as a side effect of this call. Lookup does NOT
// call purgeExpiredLocked on entry — that would turn expired tokens into
// not-found tokens, collapsing the 410/404 distinction the HTTP handler
// relies on. Instead, only the looked-up token is checked for expiry.
func (s *ImportStaging) Lookup(token string) (StagingEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[token]
	if !ok {
		return StagingEntry{}, ErrStagingNotFound
	}

	if s.now().After(entry.expiresAt) {
		delete(s.entries, token)
		return StagingEntry{}, ErrStagingExpired
	}

	return entry, nil
}

// Remove deletes a staging entry. No-op if the token is unknown.
func (s *ImportStaging) Remove(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, token)
}

// AdvanceTime shifts the staging clock by d. Test-only — production code
// uses time.Now. Allows deterministic TTL tests without sleeping.
func (s *ImportStaging) AdvanceTime(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	base := s.now()
	s.now = func() time.Time { return base.Add(d) }
}

// purgeExpiredLocked removes expired entries. Caller must hold s.mu.
func (s *ImportStaging) purgeExpiredLocked() {
	now := s.now()
	for token, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, token)
		}
	}
}

// newToken returns a random 16-byte hex string. Panics only on rand failure,
// which is not recoverable for an upload-staging token.
func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure indicates a broken system PRNG; staging cannot
		// proceed safely. This is a programmer-visible fatal, not a user error.
		panic("import staging: crypto/rand failed: " + err.Error())
	}

	return hex.EncodeToString(b)
}

// Sentinel staging errors. IsNotFound and IsExpired let callers branch on
// the distinct HTTP status codes (404 vs 410) without importing the errors
// directly.
var (
	ErrStagingNotFound = errors.New("staging token not found")
	ErrStagingExpired  = errors.New("staging token expired")
)

// IsNotFound reports whether err is the not-found sentinel.
func IsNotFound(err error) bool { return errors.Is(err, ErrStagingNotFound) }

// IsExpired reports whether err is the expired sentinel.
func IsExpired(err error) bool { return errors.Is(err, ErrStagingExpired) }
