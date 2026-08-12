package httpapi

import (
	"testing"
	"time"
)

func TestIPRateLimiter_Allow(t *testing.T) {
	t.Parallel()

	l := newIPRateLimiter(2, time.Minute)
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
