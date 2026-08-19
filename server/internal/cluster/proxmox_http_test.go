package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApiBase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bare host, no suffix", "https://192.168.1.1:8006", "https://192.168.1.1:8006/api2/json"},
		{"already has suffix", "https://192.168.1.1:8006/api2/json", "https://192.168.1.1:8006/api2/json"},
		{"trailing slash, no suffix", "https://192.168.1.1:8006/", "https://192.168.1.1:8006/api2/json"},
		{"trailing slash, has suffix", "https://192.168.1.1:8006/api2/json/", "https://192.168.1.1:8006/api2/json"},
		{"surrounding whitespace", "  https://192.168.1.1:8006  ", "https://192.168.1.1:8006/api2/json"},
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

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, false)

	_, err := rest.do(context.Background(), http.MethodGet, "/nodes", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProxmoxRESTClient_Do_Unreachable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // closed before use: every request now fails to connect

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, false)

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

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, false)

	_, err := rest.do(context.Background(), http.MethodPost, "/access/users", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if got := err.Error(); !strings.Contains(got, "password: too short") {
		t.Fatalf("error %q should surface the field message", got)
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

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, false)
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

	rest := newProxmoxREST(srv.URL, testTokenName, testTokenVal, false).withTicket("tix", "csrf-value")
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
