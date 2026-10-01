package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"strconv"
	"testing"
	"time"
)

func TestIPRateLimiter_Allow(t *testing.T) {
	t.Parallel()

	l := newIPRateLimiter(2, time.Minute, 0, nil)
	allowed := func(l *ipRateLimiter, ip string, now time.Time) bool {
		ok, _ := l.allow(ip, now)

		return ok
	}
	base := time.Now()

	if !allowed(l, "1.2.3.4", base) {
		t.Fatal("1st request should be allowed")
	}

	if !allowed(l, "1.2.3.4", base) {
		t.Fatal("2nd request should be allowed")
	}

	if allowed(l, "1.2.3.4", base) {
		t.Fatal("3rd request within window should be rejected")
	}

	if !allowed(l, "5.6.7.8", base) {
		t.Fatal("different IP should have its own budget")
	}

	if !allowed(l, "1.2.3.4", base.Add(2*time.Minute)) {
		t.Fatal("request after window elapses should be allowed again")
	}
}

func TestUserRateLimiter_Allow(t *testing.T) {
	t.Parallel()

	l := newUserRateLimiter(30, time.Minute, 0, nil)
	base := time.Now()

	for i := range 30 {
		allowed, _ := l.allow("alice", base)
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if allowed, _ := l.allow("alice", base); allowed {
		t.Fatal("31st request within window should be rejected")
	}

	if allowed, _ := l.allow("bob", base); !allowed {
		t.Fatal("requests from a different user should not count against alice")
	}

	// After the window elapses, alice's budget resets.
	if allowed, _ := l.allow("alice", base.Add(2*time.Minute)); !allowed {
		t.Fatal("request after window elapses should be allowed again")
	}
}

func TestUserRateLimiter_RetryAfter(t *testing.T) {
	t.Parallel()

	l := newUserRateLimiter(2, time.Minute, 0, nil)
	base := time.Now()

	if allowed, _ := l.allow("alice", base); !allowed {
		t.Fatal("1st request should be allowed")
	}

	if allowed, _ := l.allow("alice", base.Add(10*time.Second)); !allowed {
		t.Fatal("2nd request should be allowed")
	}

	allowed, retry := l.allow("alice", base.Add(20*time.Second))
	if allowed {
		t.Fatal("3rd request within window should be rejected")
	}

	// The first hit is 20s into the window, so 40s remain.
	if retry < 39*time.Second || retry > 41*time.Second {
		t.Fatalf("retryAfter = %v, want ~40s", retry)
	}
}

// Keys were only pruned when they came back, so every distinct IP or user left
// an entry behind for the life of the process.
func TestRateLimiters_EvictIdleKeys(t *testing.T) {
	t.Parallel()

	base := time.Now()
	later := base.Add(3 * time.Minute)

	ip := newIPRateLimiter(5, time.Minute, 0, nil)
	user := newUserRateLimiter(5, time.Minute, 0, nil)

	for _, key := range []string{"a", "b", "c", "d"} {
		ip.allow(key, base)
		user.allow(key, base)
	}

	ip.allow("fresh", later)
	user.allow("fresh", later)

	if got := len(ip.hits); got != 1 {
		t.Errorf("ip limiter keys = %d, want 1 (idle keys evicted)", got)
	}

	if got := len(user.hits); got != 1 {
		t.Errorf("user limiter keys = %d, want 1 (idle keys evicted)", got)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func serveFrom(handler http.Handler, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	req.RemoteAddr = remote

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestIPRateLimiter_MiddlewareRejectsOverLimit(t *testing.T) {
	t.Parallel()

	handler := newIPRateLimiter(2, time.Minute, 0, nil).middleware(okHandler())

	for i := range 2 {
		if rec := serveFrom(handler, "192.0.2.1:1000"); rec.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want 204", i+1, rec.Code)
		}
	}

	rec := serveFrom(handler, "192.0.2.1:1001")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request status = %d, want 429", rec.Code)
	}

	if rec := serveFrom(handler, "192.0.2.2:1000"); rec.Code != http.StatusNoContent {
		t.Fatalf("other IP status = %d, want 204", rec.Code)
	}
}

func TestUserRateLimiter_MiddlewareSetsRetryAfter(t *testing.T) {
	t.Parallel()

	sessions, err := auth.NewSessionManager(nil, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	authHandler := NewAuth(cluster.Fake{}, sessions, "", nil, slog.New(slog.DiscardHandler))
	handler := newUserRateLimiter(1, time.Minute, 0, nil).middleware(authHandler, okHandler())

	if rec := serveFrom(handler, "192.0.2.1:1000"); rec.Code != http.StatusNoContent {
		t.Fatalf("1st request status = %d, want 204", rec.Code)
	}

	rec := serveFrom(handler, "192.0.2.1:1001")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("2nd request status = %d, want 429", rec.Code)
	}

	seconds, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || seconds < 1 || seconds > 60 {
		t.Fatalf("Retry-After = %q, want 1..60 seconds", rec.Header().Get("Retry-After"))
	}

	var body struct {
		Code              string `json:"code"`
		RetryAfterSeconds int    `json:"retryAfterSeconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "rate_limited" || body.RetryAfterSeconds != seconds {
		t.Fatalf("body = %+v, want rate_limited with retryAfterSeconds=%d", body, seconds)
	}
}
