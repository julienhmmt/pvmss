package httpapi

import (
	"testing"
	"time"
)

func TestIPRateLimiter_Allow(t *testing.T) {
	t.Parallel()

	l := newIPRateLimiter(2, time.Minute, 0, nil)
	base := time.Now()

	if !l.allow("1.2.3.4", base) {
		t.Fatal("1st request should be allowed")
	}

	if !l.allow("1.2.3.4", base) {
		t.Fatal("2nd request should be allowed")
	}

	if l.allow("1.2.3.4", base) {
		t.Fatal("3rd request within window should be rejected")
	}

	if !l.allow("5.6.7.8", base) {
		t.Fatal("different IP should have its own budget")
	}

	if !l.allow("1.2.3.4", base.Add(2*time.Minute)) {
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
