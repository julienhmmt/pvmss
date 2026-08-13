//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
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
	"time"
)

const (
	healthStatusHealthy   = "healthy"
	healthStatusUnhealthy = "unhealthy"
)

//nolint:paralleltest // serial: shared health fixture
func TestHealth(t *testing.T) { //nolint:gocyclo // table-driven test covers all health response branches
	leakyErr := errors.New("connection refused: /tmp/pvmss.db")

	cases := []struct {
		name       string
		method     string
		pinger     fakePinger
		wantStatus int
		wantBody   httpapi.HealthResponse
	}{
		{
			name:   healthStatusHealthy,
			method: http.MethodGet,
			pinger: fakePinger{err: nil},
			wantBody: httpapi.HealthResponse{
				Status: healthStatusHealthy,
				Checks: map[string]httpapi.CheckResult{
					"database": {Status: healthStatusHealthy},
				},
			},
		},
		{
			name:       healthStatusUnhealthy,
			method:     http.MethodGet,
			pinger:     fakePinger{err: leakyErr},
			wantStatus: http.StatusServiceUnavailable,
			wantBody: httpapi.HealthResponse{
				Status: healthStatusUnhealthy,
				Checks: map[string]httpapi.CheckResult{
					"database": {Status: healthStatusUnhealthy, Detail: "database unreachable"},
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
			h := httpapi.NewHealth(c.pinger, logger, nil, 60*time.Second)

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

			//nolint:nestif // nested validation mirrors the nested health response
			if len(c.wantBody.Checks) > 0 {
				gotDB := resp.Checks["database"]

				wantDB := c.wantBody.Checks["database"]
				if gotDB.Status != wantDB.Status || gotDB.Detail != wantDB.Detail {
					t.Fatalf("database check = %+v, want %+v", gotDB, wantDB)
				}

				if c.name == healthStatusUnhealthy {
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

//nolint:paralleltest // serial: shared health fixture
func TestHealth_LogsError_WhenUnhealthy(t *testing.T) {
	var buf strings.Builder

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	h := httpapi.NewHealth(fakePinger{err: errors.New("boom")}, logger, nil, 60*time.Second)

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

func (f fakePinger) Ping(_ context.Context) error {
	if f.err != nil {
		return f.err
	}

	return nil
}

// --- T021 (US3): checks.clusters aggregate + demoMode ---

// fakeFreshnessChecker implements httpapi.ClusterFreshnessChecker for tests.
type fakeFreshnessChecker struct {
	clusters []httpapi.ClusterFreshness
	demoMode bool
}

func (f fakeFreshnessChecker) Clusters() []httpapi.ClusterFreshness {
	return f.clusters
}

func (f fakeFreshnessChecker) DemoMode() bool {
	return f.demoMode
}

//nolint:paralleltest // serial: shared health fixture
func TestHealth_ClustersAggregate(t *testing.T) { //nolint:gocyclo // table-driven test covers all cluster aggregate branches
	cases := []struct {
		name             string
		pinger           fakePinger
		freshness        fakeFreshnessChecker
		wantStatus       int
		wantClustersStat string
		wantClustersDtl  string
		wantDemoMode     bool
	}{
		{
			name:             "all clusters fresh + database healthy → clusters healthy, no detail",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: "a", RefreshedAt: time.Now()}, {Name: "b", RefreshedAt: time.Now()}}},
			wantStatus:       http.StatusOK,
			wantClustersStat: "healthy",
			wantClustersDtl:  "",
			wantDemoMode:     false,
		},
		{
			name:             "one cluster stale → clusters unhealthy, detail is a count not a name",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: "alpha", RefreshedAt: time.Now()}, {Name: "beta", RefreshedAt: time.Now().Add(-10 * time.Minute)}}},
			wantStatus:       http.StatusOK,
			wantClustersStat: "unhealthy",
			wantClustersDtl:  "1 of 2 clusters unreachable",
			wantDemoMode:     false,
		},
		{
			name:             "database unhealthy → 503, top-level unhealthy (unchanged T00 rule)",
			pinger:           fakePinger{err: errors.New("db down")},
			freshness:        fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: "a", RefreshedAt: time.Now()}}},
			wantStatus:       http.StatusServiceUnavailable,
			wantClustersStat: "healthy",
			wantDemoMode:     false,
		},
		{
			name:             "demoMode true when ClusterSource is fake",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: "demo", RefreshedAt: time.Now()}}, demoMode: true},
			wantStatus:       http.StatusOK,
			wantClustersStat: "healthy",
			wantDemoMode:     true,
		},
		{
			name:             "zero clusters configured → clusters healthy (no stale possible)",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: nil},
			wantStatus:       http.StatusOK,
			wantClustersStat: "healthy",
			wantDemoMode:     false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			h := httpapi.NewHealth(c.pinger, logger, c.freshness, 60*time.Second)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/health", nil)
			h.ServeHTTP(w, r)

			if w.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, c.wantStatus)
			}

			body, _ := io.ReadAll(w.Result().Body)
			var resp httpapi.HealthResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp.DemoMode != c.wantDemoMode {
				t.Fatalf("demoMode = %v, want %v", resp.DemoMode, c.wantDemoMode)
			}

			gotClusters := resp.Checks["clusters"]
			if gotClusters.Status != c.wantClustersStat {
				t.Fatalf("clusters.status = %q, want %q", gotClusters.Status, c.wantClustersStat)
			}
			if gotClusters.Detail != c.wantClustersDtl {
				t.Fatalf("clusters.detail = %q, want %q", gotClusters.Detail, c.wantClustersDtl)
			}

			// The detail must never leak a cluster name (FR-012).
			if gotClusters.Detail != "" {
				for _, cl := range c.freshness.clusters {
					if strings.Contains(gotClusters.Detail, cl.Name) {
						t.Fatalf("clusters.detail %q leaks cluster name %q", gotClusters.Detail, cl.Name)
					}
				}
			}
		})
	}
}
