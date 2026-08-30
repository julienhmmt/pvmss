//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TestClusterDownScenario reproduces the full "cluster is down" user
// experience through the real handler chain. The fake "offline-demo" cluster
// returns ErrUnreachable on every call (no network, instant, deterministic),
// standing in for a real Proxmox endpoint that is unreachable.
//
// This is the Phase-1 feedback loop for the "disconnected app" bug report.
//
//nolint:paralleltest,funlen // integration scenario: background worker + shared temp dir
func TestClusterDownScenario(t *testing.T) {
	// One cluster named "offline-demo" — the fake returns ErrUnreachable for
	// that name on every call, mirroring a single-cluster instance whose
	// Proxmox node is down.
	clusterRegistry, err := cluster.NewRegistry(cluster.SourceFake, []store.ClusterRow{{Name: "offline-demo"}})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))

	inventoryRegistry := inventory.NewRegistry(
		clusterRegistry,
		30*time.Second,
		logger,
		inventory.WithRefreshTimeout(2*time.Second),
	)

	inventoryRegistry.SetManualRefreshMinInterval(1 * time.Second)

	ctx := t.Context()

	inventoryRegistry.Start(ctx)

	// Give the worker one refresh cycle to fail (fake returns instantly).
	time.Sleep(200 * time.Millisecond)

	cfg := config.Configuration{
		DBPath:        t.TempDir() + "/test.db",
		SessionSecret: "a-session-secret-with-at-least-thirty-two-bytes",
		ClusterSource: cluster.SourceFake,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	sessions, err := auth.NewSessionManager(st, cfg.SessionSecret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte("pvmss-local-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	authHandler := httpapi.NewAuthWithRegistry(clusterRegistry, st, sessions, string(hashBytes), auth.NewTokenService(st), logger)

	freshness := testInventoryFreshness{registry: inventoryRegistry, demoMode: false}
	authHandler.SetClusterFreshnessChecker(freshness, 60*time.Second)

	// --- POST /api/v1/auth/login (user login must be unavailable when the cluster is down) ---
	userLoginResp := serveJSON(authHandler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)

	t.Logf("POST /api/v1/auth/login status=%d body=%s", userLoginResp.Code, userLoginResp.Body.String())

	var userLoginErr struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	_ = json.Unmarshal(userLoginResp.Body.Bytes(), &userLoginErr)

	if userLoginResp.Code != http.StatusServiceUnavailable || userLoginErr.Code != "cluster_unavailable" {
		t.Fatalf("user login should be rejected when cluster is down: status=%d code=%q", userLoginResp.Code, userLoginErr.Code)
	}

	// --- POST /api/v1/auth/admin-login (local admin can still connect) ---
	loginResp := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("admin login status=%d body=%s", loginResp.Code, loginResp.Body.String())
	}

	cookie := loginResp.Result().Cookies()[0]

	// --- GET /api/v1/vms (SPA's first data call) ---
	vms := httpapi.NewVMsWithRegistry(inventoryRegistry, authHandler, 100, 0, logger, st)

	vmReq := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)

	vmReq.AddCookie(cookie)

	vmResp := httptest.NewRecorder()

	vms.ServeHTTP(vmResp, vmReq)

	t.Logf("GET /api/v1/vms status=%d body=%s", vmResp.Code, vmResp.Body.String())

	var vmErr struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	_ = json.Unmarshal(vmResp.Body.Bytes(), &vmErr)

	t.Logf("SYMPTOM 1: vm list code=%q message=%q", vmErr.Code, vmErr.Message)

	// --- GET /health (SPA status banner poll) ---
	health := httpapi.NewHealth(st, logger, freshness, 60*time.Second)

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)

	healthResp := httptest.NewRecorder()

	health.ServeHTTP(healthResp, healthReq)

	t.Logf("GET /health status=%d body=%s", healthResp.Code, healthResp.Body.String())

	// --- POST /api/v1/cluster/refresh (SPA refresh button) ---
	refresher, err := inventoryRegistry.Refresher("offline-demo")
	if err != nil {
		t.Fatalf("Refresher: %v", err)
	}

	clusterRefresh := httpapi.NewClusterRefresh(refresher, logger)

	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", strings.NewReader(""))

	refreshReq.AddCookie(cookie)

	refreshResp := httptest.NewRecorder()

	start := time.Now()

	clusterRefresh.ServeHTTP(refreshResp, refreshReq)

	elapsed := time.Since(start)

	t.Logf("POST /api/v1/cluster/refresh status=%d body=%s elapsed=%s", refreshResp.Code, refreshResp.Body.String(), elapsed)

	var refreshErr struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	_ = json.Unmarshal(refreshResp.Body.Bytes(), &refreshErr)

	t.Logf("SYMPTOM 2: refresh code=%q message=%q elapsed=%s", refreshErr.Code, refreshErr.Message, elapsed)
}

type testInventoryFreshness struct {
	registry *inventory.Registry
	demoMode bool
}

func (f testInventoryFreshness) Clusters() []httpapi.ClusterFreshness {
	all := f.registry.All()

	result := make([]httpapi.ClusterFreshness, 0, len(all))

	for name, index := range all {
		refreshedAt := time.Time{}

		if index != nil {
			refreshedAt = index.RefreshedAt
		}

		result = append(result, httpapi.ClusterFreshness{Name: name, RefreshedAt: refreshedAt})
	}

	return result
}

func (f testInventoryFreshness) DemoMode() bool {
	return f.demoMode
}
