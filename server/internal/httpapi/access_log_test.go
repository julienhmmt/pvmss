package httpapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a goroutine-safe log sink: the hijack close line is written
// from the server goroutine after the client has gone.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.String()
}

func TestAccessLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method, route string
		status        int
		want          slog.Level
	}{
		{http.MethodGet, "/api/v1/vms", 200, slog.LevelDebug},
		{http.MethodHead, "/api/v1/vms", 204, slog.LevelDebug},
		{http.MethodGet, "/api/v1/vms", 302, slog.LevelDebug},
		{http.MethodPost, "/api/v1/vms", 201, slog.LevelInfo},
		{http.MethodPut, "/api/v1/x", 200, slog.LevelInfo},
		{http.MethodPatch, "/api/v1/x", 200, slog.LevelInfo},
		{http.MethodDelete, "/api/v1/x", 204, slog.LevelInfo},
		{http.MethodGet, "/api/v1/vms", 101, slog.LevelInfo},
		{http.MethodGet, "/api/v1/vms", 404, slog.LevelWarn},
		{http.MethodPost, "/api/v1/vms", 403, slog.LevelWarn},
		{http.MethodPost, "/api/v1/auth/login", 401, slog.LevelWarn},
		{http.MethodGet, "/api/v1/auth/me", 401, slog.LevelDebug},
		{http.MethodGet, "/api/v1/vms", 500, slog.LevelError},
		{http.MethodPost, "/api/v1/vms", 502, slog.LevelError},
		{http.MethodGet, "/health", 200, slog.LevelDebug},
		{http.MethodGet, "/", 200, slog.LevelDebug},
		{http.MethodGet, "/", 404, slog.LevelDebug},
	}

	for _, c := range cases {
		if got := accessLevel(c.method, c.route, c.status); got != c.want {
			t.Errorf("%s %s %d = %v, want %v", c.method, c.route, c.status, got, c.want)
		}
	}
}

func accessLines(t *testing.T, buf *syncBuffer) []map[string]any {
	t.Helper()

	var out []map[string]any

	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}

		m := map[string]any{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad log line %q: %v", line, err)
		}

		out = append(out, m)
	}

	return out
}

func newAccessFixture(mux *http.ServeMux) (http.Handler, *syncBuffer) {
	buf := &syncBuffer{}

	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return withRequestID(log, withAccessLog(1, mux)), buf
}

func TestAccessLog_RouteIsPatternAndNoSecrets(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/vms/{cluster}/{vmid}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	h, buf := newAccessFixture(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/c1/101?token=SEKRET-Q&x=1", nil)
	req.Header.Set("Authorization", "Bearer SEKRET-H")
	req.Header.Set("Cookie", "session=SEKRET-C")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:5555"
	h.ServeHTTP(httptest.NewRecorder(), req)

	lines := accessLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %s", len(lines), buf.String())
	}

	l := lines[0]
	if l["route"] != "/api/v1/vms/{cluster}/{vmid}" || l["method"] != "GET" || l["status"] != float64(200) || l["bytes"] != float64(5) {
		t.Errorf("fields = %v", l)
	}

	if l["cluster"] != "c1" || l["vmid"] != float64(101) {
		t.Errorf("path values missing: %v", l)
	}

	if l["clientIp"] != "10.0.0.1" || l["requestId"] == nil || l["durationMs"] == nil {
		t.Errorf("fields = %v", l)
	}

	if strings.Contains(buf.String(), "SEKRET") || strings.Contains(buf.String(), "x=1") {
		t.Errorf("secret or query leaked: %s", buf.String())
	}
}

func TestAccessLog_SetErrorYieldsOneErrorLine(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/boom", func(w http.ResponseWriter, _ *http.Request) {
		SetError(w, errors.New("db exploded"))
		w.WriteHeader(http.StatusInternalServerError)
	})

	h, buf := newAccessFixture(mux)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/boom", nil))

	lines := accessLines(t, buf)
	if len(lines) != 1 || lines[0]["level"] != "ERROR" || lines[0]["error"] != "db exploded" {
		t.Fatalf("lines = %v", lines)
	}
}

func TestAccessLog_HijackLogsUpgradeAndClose(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ws", func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("Hijack through wrapper: %v", err)
			return
		}

		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))
		_ = conn.Close()
	})

	h, buf := newAccessFixture(mux)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	_, _ = c.Write([]byte("GET /api/v1/ws HTTP/1.1\r\nHost: x\r\n\r\n"))
	_, _ = bufio.NewReader(c).ReadString('\n')
	_ = c.Close()

	deadline := time.Now().Add(5 * time.Second)
	for strings.Count(buf.String(), "\n") < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	lines := accessLines(t, buf)
	if len(lines) != 2 || lines[0]["msg"] != "websocket upgraded" || lines[0]["status"] != float64(101) || lines[1]["msg"] != "websocket closed" {
		t.Fatalf("lines = %v", lines)
	}
}
