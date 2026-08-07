//nolint:goconst // test fixture strings
package httpapi_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	leakyErr := errors.New("connection refused: /tmp/pvmss.db")

	cases := []struct {
		name       string
		method     string
		pinger     fakePinger
		wantStatus int
		wantBody   httpapi.HealthResponse
	}{
		{
			name:   "healthy",
			method: http.MethodGet,
			pinger: fakePinger{err: nil},
			wantBody: httpapi.HealthResponse{
				Status: "healthy",
				Checks: map[string]httpapi.CheckResult{
					"database": {Status: "healthy"},
				},
			},
		},
		{
			name:       "unhealthy",
			method:     http.MethodGet,
			pinger:     fakePinger{err: leakyErr},
			wantStatus: http.StatusServiceUnavailable,
			wantBody: httpapi.HealthResponse{
				Status: "unhealthy",
				Checks: map[string]httpapi.CheckResult{
					"database": {Status: "unhealthy", Detail: "database unreachable"},
				},
			},
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			pinger:     fakePinger{err: nil},
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			h := httpapi.NewHealth(c.pinger, logger)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(c.method, "/health", nil)
			h.ServeHTTP(w, r)

			wantStatus := http.StatusOK
			if c.wantStatus != 0 {
				wantStatus = c.wantStatus
			}

			if w.Code != wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, wantStatus)
			}

			body, _ := io.ReadAll(w.Result().Body)

			if c.method == http.MethodPost {
				// The response must include an Allow header for 405.
				if !strings.Contains(w.Header().Get("Allow"), "GET") {
					t.Fatalf("missing Allow header for 405: %q", w.Header().Get("Allow"))
				}

				return
			}

			var resp httpapi.HealthResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp.Status != c.wantBody.Status {
				t.Fatalf("status = %q, want %q", resp.Status, c.wantBody.Status)
			}

			if len(c.wantBody.Checks) > 0 {
				gotDB := resp.Checks["database"]

				wantDB := c.wantBody.Checks["database"]
				if gotDB.Status != wantDB.Status || gotDB.Detail != wantDB.Detail {
					t.Fatalf("database check = %+v, want %+v", gotDB, wantDB)
				}

				if c.name == "unhealthy" {
					if strings.Contains(gotDB.Detail, c.pinger.err.Error()) {
						t.Fatalf("detail leaks raw error: %q", gotDB.Detail)
					}

					if strings.Contains(gotDB.Detail, "/tmp/pvmss.db") {
						t.Fatalf("detail leaks path: %q", gotDB.Detail)
					}
				}
			}

			if resp.Timestamp == "" {
				t.Fatalf("timestamp is empty")
			}
		})
	}
}

func TestHealth_LogsError_WhenUnhealthy(t *testing.T) {
	var buf strings.Builder

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	h := httpapi.NewHealth(fakePinger{err: errors.New("boom")}, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	output := buf.String()
	if !strings.Contains(output, "database health check failed") {
		t.Fatalf("expected error log, got %q", output)
	}

	if !strings.Contains(output, "boom") {
		t.Fatalf("expected raw error in server log, got %q", output)
	}

	// The response detail must remain sanitized even though the server log
	// contains the raw error.
	body, _ := io.ReadAll(w.Result().Body)

	var resp httpapi.HealthResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Checks["database"].Detail != "database unreachable" {
		t.Fatalf("response detail = %q, want %q", resp.Checks["database"].Detail, "database unreachable")
	}
}

type fakePinger struct {
	err error
}

func (f fakePinger) Ping() error {
	if f.err != nil {
		return f.err
	}

	return nil
}
