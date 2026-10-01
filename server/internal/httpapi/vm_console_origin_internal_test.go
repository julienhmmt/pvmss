package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
)

const consoleTestHost = "pvmss.example.com"

// The console WebSocket carries a live VM session, so the Origin check is the
// only defence against cross-site WebSocket hijacking.
func TestIsConsoleOriginAllowed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"same host over https", "https://" + consoleTestHost, consoleTestHost, true},
		{"same host and port", "http://localhost:5173", "localhost:5173", true},
		{"ws scheme is accepted", "wss://" + consoleTestHost, consoleTestHost, true},
		{"missing origin", "", consoleTestHost, false},
		{"other host", "https://evil.example.net", consoleTestHost, false},
		{"same host other port", "https://" + consoleTestHost + ":8443", consoleTestHost, false},
		{"unparseable origin", "http://[::1", consoleTestHost, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			req.Host = tc.host

			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			if got := isConsoleOriginAllowed(req); got != tc.want {
				t.Fatalf("isConsoleOriginAllowed(origin=%q, host=%q) = %v, want %v", tc.origin, tc.host, got, tc.want)
			}
		})
	}
}

func TestIsNormalClose(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true},
		{"eof", fmt.Errorf("read: %w", io.EOF), true},
		{"normal closure", websocket.CloseError{Code: websocket.StatusNormalClosure}, true},
		{"going away", websocket.CloseError{Code: websocket.StatusGoingAway}, true},
		{"abnormal closure", websocket.CloseError{Code: websocket.StatusAbnormalClosure}, false},
		{"policy violation", websocket.CloseError{Code: websocket.StatusPolicyViolation}, false},
		{"internal error", websocket.CloseError{Code: websocket.StatusInternalError}, false},
		{"unrelated error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isNormalClose(tc.err); got != tc.want {
				t.Fatalf("isNormalClose(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
