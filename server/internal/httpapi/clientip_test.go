package httpapi

import (
	"net/http"
	"testing"
)

func TestClientIP_NoProxy_UsesRemoteAddr(t *testing.T) {
	t.Parallel()

	r := &http.Request{RemoteAddr: "203.0.113.5:54321"}
	if got := clientIP(r, 0); got != "203.0.113.5" {
		t.Fatalf("clientIP hops=0 = %q, want 203.0.113.5", got)
	}
}

func TestClientIP_NoXFF_UsesRemoteAddr(t *testing.T) {
	t.Parallel()

	r := &http.Request{RemoteAddr: "203.0.113.5:54321"}
	if got := clientIP(r, 1); got != "203.0.113.5" {
		t.Fatalf("clientIP hops=1 no XFF = %q, want 203.0.113.5", got)
	}
}

func TestClientIP_SingleHop_SelectsLastXFFEntry(t *testing.T) {
	t.Parallel()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:54321",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.5"}},
	}
	// One trusted proxy: trust the rightmost entry (the proxy's claim).
	if got := clientIP(r, 1); got != "203.0.113.5" {
		t.Fatalf("clientIP hops=1 single XFF = %q, want 203.0.113.5", got)
	}
}

func TestClientIP_TwoHops_SelectsFirstEntry(t *testing.T) {
	t.Parallel()

	r := &http.Request{
		RemoteAddr: "10.0.0.2:54321",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.5, 10.0.0.1"}},
	}
	// Two trusted proxies: trust both hops, take the leftmost (original client).
	if got := clientIP(r, 2); got != "203.0.113.5" {
		t.Fatalf("clientIP hops=2 = %q, want 203.0.113.5", got)
	}
}

func TestClientIP_OneHopTwoEntries_SelectsProxyClaim(t *testing.T) {
	t.Parallel()

	r := &http.Request{
		RemoteAddr: "10.0.0.2:54321",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.5, 10.0.0.1"}},
	}
	// One trusted proxy: take the rightmost entry (the trusted proxy's IP).
	if got := clientIP(r, 1); got != "10.0.0.1" {
		t.Fatalf("clientIP hops=1 two XFF = %q, want 10.0.0.1", got)
	}
}

func TestClientIP_HopsExceedsXFFLength_FallsBackToFirst(t *testing.T) {
	t.Parallel()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:54321",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.5"}},
	}
	// hops=3 but only 1 entry: clamp to index 0.
	if got := clientIP(r, 3); got != "203.0.113.5" {
		t.Fatalf("clientIP hops=3 short XFF = %q, want 203.0.113.5", got)
	}
}

func TestClientIP_EmptyXFF_UsesRemoteAddr(t *testing.T) {
	t.Parallel()

	r := &http.Request{
		RemoteAddr: "203.0.113.5:54321",
		Header:     http.Header{"X-Forwarded-For": []string{""}},
	}
	if got := clientIP(r, 1); got != "203.0.113.5" {
		t.Fatalf("clientIP empty XFF = %q, want 203.0.113.5", got)
	}
}

func TestClientIP_MalformedRemoteAddr_ReturnsRaw(t *testing.T) {
	t.Parallel()

	// No colon → SplitHostPort fails → return raw RemoteAddr.
	r := &http.Request{RemoteAddr: "localhost"}
	if got := clientIP(r, 0); got != "localhost" {
		t.Fatalf("clientIP malformed RemoteAddr = %q, want localhost", got)
	}
}

func TestClientIP_WhitespaceInXFF_IsTrimmed(t *testing.T) {
	t.Parallel()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:54321",
		Header:     http.Header{"X-Forwarded-For": []string{" 203.0.113.5 , 10.0.0.1 "}},
	}
	if got := clientIP(r, 2); got != "203.0.113.5" {
		t.Fatalf("clientIP with whitespace = %q, want 203.0.113.5", got)
	}
}
