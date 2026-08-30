package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestApiBase(t *testing.T) {
	t.Parallel()

	const (
		apiBaseHost = "https://192.168.1.1:8006"
		apiBaseWant = "https://192.168.1.1:8006/api2/json"
	)

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bare host, no suffix", apiBaseHost, apiBaseWant},
		{"already has suffix", apiBaseWant, apiBaseWant},
		{"trailing slash, no suffix", apiBaseHost + "/", apiBaseWant},
		{"trailing slash, has suffix", apiBaseWant + "/", apiBaseWant},
		{"surrounding whitespace", "  " + apiBaseHost + "  ", apiBaseWant},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := apiBase(tc.raw); got != tc.want {
				t.Errorf("apiBase(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestProxmoxRESTClient_Do_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProxmoxRESTClient_Do_Unreachable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // closed before use: every request now fails to connect

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false)).withNoRetry()

	_, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil)
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestProxmoxRESTClient_Do_ErrorBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"data":null,"errors":{"password":"too short"}}`))
	}))
	t.Cleanup(srv.Close)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodPost, "/access/users", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrClusterRejected) {
		t.Fatalf("error should satisfy ErrClusterRejected, got %v", err)
	}

	var rejection *RejectionError
	if !errors.As(err, &rejection) {
		t.Fatalf("error should be a *RejectionError, got %T", err)
	}

	if rejection.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rejection.Status, http.StatusBadRequest)
	}

	if got := err.Error(); !strings.Contains(got, "password: too short") {
		t.Fatalf("error %q should surface the field message", got)
	}
}

func TestProxmoxRESTClient_Do_RejectionExtractsTopLevelMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"data":null,"message":"VM is locked (backup)"}`))
	}))
	t.Cleanup(srv.Close)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodPost, "/nodes/n1/qemu/100/status/start", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var rejection *RejectionError
	if !errors.As(err, &rejection) {
		t.Fatalf("error should be a *RejectionError, got %T", err)
	}

	if rejection.Status != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rejection.Status, http.StatusInternalServerError)
	}

	if rejection.Message != "VM is locked (backup)" {
		t.Errorf("message = %q, want the top-level message", rejection.Message)
	}
}

func TestProxmoxRESTClient_Do_RejectionCarriesStatusForAuthErrors(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"data":null,"message":"no such API token: user@pve!tokenname"}`))
	}))
	t.Cleanup(srv.Close)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodGet, "/cluster/resources", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var rejection *RejectionError
	if !errors.As(err, &rejection) {
		t.Fatalf("error should be a *RejectionError, got %T", err)
	}

	// The cluster layer keeps the raw message (the HTTP layer decides whether
	// to surface it — a 401 body can name the token).
	if rejection.Status != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rejection.Status, http.StatusUnauthorized)
	}

	if !strings.Contains(rejection.Message, "tokenname") {
		t.Errorf("message = %q, want the raw body retained for the http layer", rejection.Message)
	}
}

func TestProxmoxRESTClient_Authenticate_UsesTokenByDefault(t *testing.T) {
	t.Parallel()

	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))
	if _, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil); err != nil {
		t.Fatalf("do: %v", err)
	}

	want := "PVEAPIToken=" + testTokenName + "=" + testTokenVal
	if gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestProxmoxRESTClient_Authenticate_UsesTicketWhenSet(t *testing.T) {
	t.Parallel()

	var gotCookie, gotCSRF string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("PVEAuthCookie"); err == nil {
			gotCookie = cookie.Value
		}

		gotCSRF = r.Header.Get("CSRFPreventionToken")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	t.Cleanup(srv.Close)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false)).withTicket("tix", "csrf-value")
	if _, err := rest.do(context.Background(), http.MethodPut, "/access/password", nil); err != nil {
		t.Fatalf("do: %v", err)
	}

	if gotCookie != "tix" {
		t.Errorf("cookie = %q, want %q", gotCookie, "tix")
	}

	if gotCSRF != "csrf-value" {
		t.Errorf("csrf header = %q, want %q", gotCSRF, "csrf-value")
	}
}

// --- Ticket 07: GET-only retry tests ---

// retryTestServer builds an httptest.Server whose handler consults a
// per-request response plan. Each call increments the shared counter so tests
// can assert exact attempt counts. The plan is a slice of status codes; the
// last entry is reused if the counter exceeds the plan length.
func retryTestServer(t *testing.T, plan []int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		idx := int(calls.Add(1)) - 1

		status := http.StatusOK
		if idx < len(plan) {
			status = plan[idx]
		} else if len(plan) > 0 {
			status = plan[len(plan)-1]
		}

		w.WriteHeader(status)

		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)

	return srv, &calls
}

func TestDo_RetriesGET_On503Then200(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusServiceUnavailable, http.StatusOK}, `{"data":[]}`)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	raw, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil)
	if err != nil {
		t.Fatalf("do: %v", err)
	}

	if string(raw) != "[]" {
		t.Errorf("raw = %q, want `[]`", string(raw))
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("server calls = %d, want 2", got)
	}
}

func TestDo_RetriesGET_Persistent503_Exactly3Calls(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusServiceUnavailable}, "")

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil)
	if err == nil {
		t.Fatal("expected error from persistent 503, got nil")
	}

	if got := calls.Load(); got != 3 {
		t.Errorf("server calls = %d, want 3 (1 initial + 2 retries)", got)
	}
}

func TestDo_DoesNotRetryPOST_CouldDoubleProvision(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusServiceUnavailable}, "")

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodPost, "/nodes/qemu", nil)
	if err == nil {
		t.Fatal("expected error from 503 on POST, got nil")
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("server calls = %d, want 1 (POST must never be retried)", got)
	}
}

func TestDo_DoesNotRetry404(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusNotFound}, "")

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	_, err := rest.do(context.Background(), http.MethodGet, "/nodes/missing", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("server calls = %d, want 1 (404 is not transient)", got)
	}
}

func TestDo_RetriesGET_On429Then200(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusTooManyRequests, http.StatusOK}, `{"data":null}`)

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	if _, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil); err != nil {
		t.Fatalf("do: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("server calls = %d, want 2", got)
	}
}

func TestDo_NoRetryFlag_SkipsRetry(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusServiceUnavailable}, "")

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false)).withNoRetry()

	_, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil)
	if err == nil {
		t.Fatal("expected error from 503 with noRetry, got nil")
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("server calls = %d, want 1 (noRetry short-circuits)", got)
	}
}

func TestDo_ContextCancellationDuringBackoff_ReturnsPromptly(t *testing.T) {
	t.Parallel()

	srv, calls := retryTestServer(t, []int{http.StatusServiceUnavailable}, "")

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()

	_, err := rest.do(ctx, http.MethodGet, "/nodes", nil)

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}

	// The first call returns 503 immediately, then backoff (250ms) starts.
	// With a 50ms context deadline, the backoff select should fire ctx.Done()
	// well before 250ms — total elapsed should be far under 1s.
	if elapsed > time.Second {
		t.Errorf("elapsed = %v, want < 1s (context should cancel backoff promptly)", elapsed)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("server calls = %d, want 1 (only the first attempt before ctx cancel)", got)
	}
}

func TestDo_ConnectionReuseAcrossCalls(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	t.Cleanup(srv.Close)

	// Use a single shared *http.Client so the Transport pool is reused.
	client := newProxmoxHTTPClient(false)
	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, client)

	for range 5 {
		if _, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil); err != nil {
			t.Fatalf("do: %v", err)
		}
	}

	if got := calls.Load(); got != 5 {
		t.Errorf("server calls = %d, want 5", got)
	}
}
