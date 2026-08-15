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
func TestHealth(t *testing.T) {
	leakyErr := errors.New("connection refused: /tmp/pvmss.db")

	healthCases := []healthCase{
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

	for _, c := range healthCases {
		t.Run(c.name, func(t *testing.T) {
			runHealthCase(t, c)
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

// clustersCase is a single row of the TestHealth_ClustersAggregate table.
type clustersCase struct {
	name             string
	pinger           fakePinger
	freshness        fakeFreshnessChecker
	wantStatus       int
	wantClustersStat string
	wantClustersDtl  string
	wantDemoMode     bool
}

// healthCase is a single row of the TestHealth table (basic database + method check).
type healthCase struct {
	name       string
	method     string
	pinger     fakePinger
	wantStatus int
	wantBody   httpapi.HealthResponse
}

// runHealthCase executes one TestHealth table row: build the request, invoke the
// health handler, and assert the status, Allow header (for 405), and the decoded
// body. Extracted from the table loop to keep TestHealth's Cognitive Complexity
// under the go:S3776 threshold.
func runHealthCase(t *testing.T, c healthCase) {
	t.Helper()

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

	if len(c.wantBody.Checks) > 0 {
		assertDatabaseCheck(t, resp.Checks["database"], c)
	}

	if resp.Timestamp == "" {
		t.Fatalf("timestamp is empty")
	}
}

// assertDatabaseCheck validates the decoded "database" health check against the
// expected values, and (for the unhealthy case) confirms no raw error or path
// detail leaks into the sanitized response body.
func assertDatabaseCheck(t *testing.T, gotDB httpapi.CheckResult, c healthCase) {
	t.Helper()

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

// runClustersCase executes one TestHealth_ClustersAggregate row: it builds the
// request, invokes the health handler, decodes the response, and asserts the
// status, clusters check, and demoMode. Extracted from the table loop to keep
// the test's Cognitive Complexity under the go:S3776 threshold.
func runClustersCase(t *testing.T, tc clustersCase) {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	h := httpapi.NewHealth(tc.pinger, logger, tc.freshness, 60*time.Second)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(w, r)

	if w.Code != tc.wantStatus {
		t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
	}

	body, _ := io.ReadAll(w.Result().Body)

	var resp httpapi.HealthResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.DemoMode != tc.wantDemoMode {
		t.Fatalf("demoMode = %v, want %v", resp.DemoMode, tc.wantDemoMode)
	}

	gotClusters := resp.Checks["clusters"]
	if gotClusters.Status != tc.wantClustersStat {
		t.Fatalf("clusters.status = %q, want %q", gotClusters.Status, tc.wantClustersStat)
	}

	if gotClusters.Detail != tc.wantClustersDtl {
		t.Fatalf("clusters.detail = %q, want %q", gotClusters.Detail, tc.wantClustersDtl)
	}

	// The detail must never leak a cluster name (FR-012).
	if gotClusters.Detail != "" {
		for _, cl := range tc.freshness.clusters {
			if strings.Contains(gotClusters.Detail, cl.Name) {
				t.Fatalf("clusters.detail %q leaks cluster name %q", gotClusters.Detail, cl.Name)
			}
		}
	}
}

//nolint:paralleltest // serial: shared health fixture
func TestHealth_ClustersAggregate(t *testing.T) {
	cases := []clustersCase{
		{
			name:             "all clusters fresh + database healthy → clusters healthy, no detail",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: "a", RefreshedAt: time.Now()}, {Name: "b", RefreshedAt: time.Now()}}},
			wantStatus:       http.StatusOK,
			wantClustersStat: healthStatusHealthy,
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
			wantClustersStat: healthStatusHealthy,
			wantDemoMode:     false,
		},
		{
			name:             "demoMode true when ClusterSource is fake",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: "demo", RefreshedAt: time.Now()}}, demoMode: true},
			wantStatus:       http.StatusOK,
			wantClustersStat: healthStatusHealthy,
			wantDemoMode:     true,
		},
		{
			name:             "zero clusters configured → clusters healthy (no stale possible)",
			pinger:           fakePinger{err: nil},
			freshness:        fakeFreshnessChecker{clusters: nil},
			wantStatus:       http.StatusOK,
			wantClustersStat: healthStatusHealthy,
			wantDemoMode:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runClustersCase(t, tc)
		})
	}
}
