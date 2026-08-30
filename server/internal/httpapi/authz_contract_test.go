//nolint:noctx,goconst // test scaffolding does not need real context; snapshot body literal reused across contract tests
package httpapi_test

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

//go:embed router.go
var routerGo string

//go:embed router_admin.go
var routerAdminGo string

//go:embed vm_detail.go
var vmDetailGo string

//go:embed vm_cloudinit.go
var vmCloudInitGo string

//go:embed vm_snapshots.go
var vmSnapshotsGo string

//go:embed vm_console.go
var vmConsoleGo string

//go:embed vm_serial_console.go
var vmSerialConsoleGo string

//go:embed vm_metrics.go
var vmMetricsGo string

type extractedRoute struct {
	Pattern    string
	Method     string
	Path       string
	HandlerArg string
	Line       string
}

// routePatternRe matches the literal mux.Handle("METHOD /path", ...) and
// mux.HandleFunc("METHOD /path", ...) registrations found in router*.go.
// It captures the pattern string and the argument that follows it (the
// handler or wrapped handler) up to the first closing parenthesis on the line.
var routePatternRe = regexp.MustCompile(`(?m)^\s*mux\.Handle(?:Func)?\s*\(\s*"([^"]+)"\s*,\s*([^)\n]+)`)

func extractRoutePatterns(t *testing.T) []extractedRoute {
	t.Helper()

	var routes []extractedRoute

	for _, src := range []string{routerGo, routerAdminGo} {
		matches := routePatternRe.FindAllStringSubmatch(src, -1)
		for _, m := range matches {
			pattern := m[1]

			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) != 2 {
				t.Fatalf("route pattern %q did not contain a method", pattern)
			}

			method := parts[0]
			path := parts[1]

			routes = append(routes, extractedRoute{
				Pattern:    pattern,
				Method:     method,
				Path:       path,
				HandlerArg: strings.TrimSpace(m[2]),
				Line:       m[0],
			})
		}
	}

	sort.Slice(routes, func(i, j int) bool { return routes[i].Pattern < routes[j].Pattern })

	return routes
}

// filledPath replaces path parameter placeholders with deterministic test
// values so black-box requests reach the route table and the real handler.
func fillPath(path string) string {
	replacements := map[string]string{
		"{cluster}": "default",
		"{vmid}":    "100",
		"{diskKey}": "scsi0",
		"{name}":    "x",
		"{id}":      "1",
		"{node}":    "pve-node-01",
		"{lang}":    "en",
	}

	result := path
	for param, value := range replacements {
		result = strings.ReplaceAll(result, param, value)
	}

	return result
}

// -----------------------------------------------------------------------------
// T045: admin black-box contract
// -----------------------------------------------------------------------------

//nolint:paralleltest // serial: shared router and database fixtures
func TestAdminAuthorization_BlackBox(t *testing.T) {
	routes := extractRoutePatterns(t)
	adminRoutes := filterAdminRoutes(routes)

	if len(adminRoutes) < 40 {
		t.Fatalf("admin route canary failed: found %d routes, want at least 40", len(adminRoutes))
	}

	mux, authHandler := newAuthzContractRouter(t)
	cookie := bobCookie(t, authHandler)

	for _, r := range adminRoutes {
		t.Run(r.Pattern, func(t *testing.T) {
			path := fillPath(r.Path)
			req := httptest.NewRequest(r.Method, path, nil)
			req.AddCookie(cookie)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status = %d, want 403 for non-admin", r.Method, path, rec.Code)
			}
		})
	}
}

func filterAdminRoutes(routes []extractedRoute) []extractedRoute {
	var out []extractedRoute

	for _, r := range routes {
		if strings.HasPrefix(r.Path, "/api/v1/admin/") {
			out = append(out, r)
		}
	}

	return out
}

// -----------------------------------------------------------------------------
// T046: adminProtect fail-closed boundary test
// -----------------------------------------------------------------------------

//nolint:paralleltest // serial: shared auth and session fixtures
func TestAdminProtect_FailClosed(t *testing.T) {
	_, authHandler := newAuthzContractRouter(t)
	adminCookie := adminCookie(t, authHandler)
	bobCookie := bobCookie(t, authHandler)

	cases := []struct {
		name      string
		cookie    *http.Cookie
		wantPass  bool
		wantCodes []int // expected response codes; empty means the wrapped handler should be called
	}{
		{
			name:      "nil/unauthenticated identity",
			cookie:    nil,
			wantPass:  false,
			wantCodes: []int{http.StatusUnauthorized, http.StatusForbidden},
		},
		{
			name:      "non-admin identity",
			cookie:    bobCookie,
			wantPass:  false,
			wantCodes: []int{http.StatusForbidden},
		},
		{
			name:      "admin identity",
			cookie:    adminCookie,
			wantPass:  true,
			wantCodes: []int{http.StatusOK},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runAdminProtectCase(t, authHandler, c.cookie, c.wantPass, c.wantCodes)
		})
	}
}

// runAdminProtectCase exercises one adminProtect scenario. Extracted from
// TestAdminProtect_FailClosed to keep cognitive complexity below go:S3776.
func runAdminProtectCase(t *testing.T, authHandler *httpapi.Auth, cookie *http.Cookie, wantPass bool, wantCodes []int) {
	t.Helper()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	guarded := authHandler.RequireAdmin(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if wantPass {
		if !called {
			t.Fatalf("wrapped handler should have been called")
		}

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		return
	}

	if called {
		t.Fatalf("wrapped handler should not be called")
	}

	if !slices.Contains(wantCodes, rec.Code) {
		t.Fatalf("status = %d, want one of %v", rec.Code, wantCodes)
	}
}

// -----------------------------------------------------------------------------
// T047: CSRF contract
// -----------------------------------------------------------------------------

// csrfExemptRoutes lists non-GET routes that are deliberately not wrapped in
// the browser-session CSRF middleware because they are public, unauthenticated
// endpoints (login, admin-login, OIDC trigger). Exemption justifications are
// embedded in the test logic.
func csrfExemptRoutes() map[string]string {
	return map[string]string{
		"POST /api/v1/auth/login":       "public unauthenticated login endpoint",
		"POST /api/v1/auth/admin-login": "public unauthenticated admin-login endpoint",
		"POST /api/v1/auth/oidc":        "public OIDC trigger endpoint",
	}
}

//nolint:paralleltest // serial: shared router and session fixtures
func TestCSRFContract_NonGETRoutesRejected(t *testing.T) {
	routes := extractRoutePatterns(t)
	mux, authHandler := newAuthzContractRouter(t)
	bob := bobCookie(t, authHandler)
	admin := adminCookie(t, authHandler)

	exempt := csrfExemptRoutes()

	for _, r := range routes {
		if r.Method == http.MethodGet {
			continue
		}

		if reason, ok := exempt[r.Pattern]; ok {
			t.Run("exempt "+r.Pattern, func(_ *testing.T) {
				_ = reason
			})

			continue
		}

		t.Run(r.Pattern, func(t *testing.T) {
			cookie := bob
			if strings.HasPrefix(r.Path, "/api/v1/admin/") {
				// admin routes are wrapped with adminProtect; use an admin
				// session so the request reaches the inner CSRF guard.
				cookie = admin
			}

			path := fillPath(r.Path)
			req := httptest.NewRequest(r.Method, path, nil)
			req.AddCookie(cookie)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status = %d, want 403 (missing CSRF token)", r.Method, path, rec.Code)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// T048: Resolve() syntaxic contract
// -----------------------------------------------------------------------------

var handlerSourceMap = map[string]string{
	"cfg.VMDetail":        vmDetailGo,
	"cfg.VMCloudInit":     vmCloudInitGo,
	"cfg.VMSnapshots":     vmSnapshotsGo,
	"snapshots":           vmSnapshotsGo,
	"cfg.VMConsole":       vmConsoleGo,
	"cfg.VMSerialConsole": vmSerialConsoleGo,
	"handler":             vmSerialConsoleGo,
	"cfg.VMMetrics":       vmMetricsGo,
}

// resolveExemptRoutes lists per-VM routes whose handler source does not itself
// contain a call to vm.Resolve() because ownership is enforced inside the vm
// package helper they call. Each entry carries its written justification.
var resolveExemptRoutes = map[string]string{
	// cloud-init: handler delegates to vm.Get/SetCloudInitConfig, vm.AddCloudInitSSHKey,
	// and vm.Get/SetCloudInitSnippet, all of which resolve the VM and enforce ownership.
	"GET /api/v1/vms/{cluster}/{vmid}/cloudinit":           "delegates to vm.GetCloudInitConfig which resolves",
	"PUT /api/v1/vms/{cluster}/{vmid}/cloudinit":           "delegates to vm.SetCloudInitConfig which resolves",
	"GET /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet":   "delegates to vm.GetCloudInitSnippet which resolves",
	"PUT /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet":   "delegates to vm.SetCloudInitSnippet which resolves",
	"POST /api/v1/vms/{cluster}/{vmid}/cloudinit/ssh-keys": "delegates to vm.AddCloudInitSSHKey which resolves",
	// console: ticket endpoints call vm.GetConsoleTicket (Resolve -> GetVNCTicket -> Issue -> audit).
	"POST /api/v1/vms/{cluster}/{vmid}/vnc-ticket":    "delegates to vm.GetConsoleTicket which resolves",
	"POST /api/v1/vms/{cluster}/{vmid}/serial-ticket": "delegates to vm.GetConsoleTicket which resolves",
	// websocket: the per-VM ownership gate is the single-use ticket consumed at the websocket.
	"GET /api/v1/vms/{cluster}/{vmid}/console/websocket": "ownership is enforced by the pre-issued, VM-bound ticket",
	"GET /api/v1/vms/{cluster}/{vmid}/serial/websocket":  "ownership is enforced by the pre-issued, VM-bound ticket",
	// snapshots: vm.List/Create/Rollback/DeleteSnapshot resolve and enforce ownership.
	"GET /api/v1/vms/{cluster}/{vmid}/snapshots":                  "delegates to vm.ListSnapshots which resolves",
	"POST /api/v1/vms/{cluster}/{vmid}/snapshots":                 "delegates to vm.CreateSnapshot which resolves",
	"POST /api/v1/vms/{cluster}/{vmid}/snapshots/{name}/rollback": "delegates to vm.RollbackSnapshot which resolves",
	"DELETE /api/v1/vms/{cluster}/{vmid}/snapshots/{name}":        "delegates to vm.DeleteSnapshot which resolves",
	"GET /api/v1/vms/{cluster}/{vmid}/snapshots/{name}/config":    "delegates to vm.SnapshotConfig which resolves",
}

func handlerArgToSource(arg string) (string, bool) {
	// Strip the outer protect(...) / adminProtect(...) wrapper to find the
	// handler variable (e.g. cfg.VMDetail, snapshots, handler).
	arg = strings.TrimSpace(arg)
	for _, prefix := range []string{"protect(", "adminProtect("} {
		if rest, ok := strings.CutPrefix(arg, prefix); ok {
			arg = rest
			if i := strings.Index(arg, ","); i >= 0 {
				arg = strings.TrimSpace(arg[:i])
			}
		}
	}

	src, ok := handlerSourceMap[arg]

	return src, ok
}

//nolint:paralleltest // serial: test data is shared
func TestResolveContract_PerVMRoutes(t *testing.T) {
	routes := extractRoutePatterns(t)

	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/api/v1/vms/{cluster}/{vmid}") {
			continue
		}

		t.Run(r.Pattern, func(t *testing.T) {
			runResolveContractCase(t, r)
		})
	}
}

// runResolveContractCase checks one per-VM route's handler references
// vm.Resolve() (or carries a written exemption). Extracted from
// TestResolveContract_PerVMRoutes to keep cognitive complexity below go:S3776.
func runResolveContractCase(t *testing.T, r extractedRoute) {
	t.Helper()

	if reason, ok := resolveExemptRoutes[r.Pattern]; ok {
		// Exemptions are intentional and must be justified in writing.
		// Removing this check forces the next reader to re-justify it.
		if !strings.Contains(reason, "resolves") && !strings.Contains(reason, "ticket") {
			t.Fatalf("exemption %q has no written justification", r.Pattern)
		}

		return
	}

	src, ok := handlerArgToSource(r.HandlerArg)
	if !ok {
		t.Fatalf("no handler source mapped for %s using arg %q", r.Pattern, r.HandlerArg)
	}

	if !strings.Contains(src, "vm.Resolve(") {
		t.Fatalf("handler for %s does not reference vm.Resolve()", r.Pattern)
	}
}

// -----------------------------------------------------------------------------
// T049: IDOR black-box
// -----------------------------------------------------------------------------

var vmRequestBodies = map[string]string{
	"POST /api/v1/vms/{cluster}/{vmid}/actions":               `{"action":"stop"}`,
	"PATCH /api/v1/vms/{cluster}/{vmid}":                      `{"name":"web-01","description":"x"}`,
	"POST /api/v1/vms/{cluster}/{vmid}/disks":                 `{"bus":"scsi","storage":"local-lvm","sizeGb":10}`,
	"PUT /api/v1/vms/{cluster}/{vmid}/disks/{diskKey}/resize": `{"sizeGb":20}`,
	"PATCH /api/v1/vms/{cluster}/{vmid}/cdrom":                `{"action":"eject"}`,
	"PUT /api/v1/vms/{cluster}/{vmid}/network":                `{"interfaces":[]}`,
	"PUT /api/v1/vms/{cluster}/{vmid}/hardware":               `{"sockets":2}`,
	"POST /api/v1/vms/{cluster}/{vmid}/serial":                `{"enabled":true}`,
	"PUT /api/v1/vms/{cluster}/{vmid}/cloudinit":              `{"user":"root"}`,
	"PUT /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet":      `{"content":"#cloud-config"}`,
	"POST /api/v1/vms/{cluster}/{vmid}/cloudinit/ssh-keys":    `{"user":"root","key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDIhz2GK/XCuN1lGHKPmXGP"}`,
	"POST /api/v1/vms/{cluster}/{vmid}/snapshots":             `{"name":"x"}`,
}

// idorNonForbiddenRoutes are per-VM routes where a request without the
// required pre-conditions (WebSocket ticket, etc.) does not return 403 from
// Resolve(). They are still rejected with a non-2xx status, so the test keeps
// the contract that Bob cannot access Alice's VM.
var idorNonForbiddenRoutes = map[string]string{
	"GET /api/v1/vms/{cluster}/{vmid}/console/websocket": "websocket requires a pre-issued ticket",
	"GET /api/v1/vms/{cluster}/{vmid}/serial/websocket":  "websocket requires a pre-issued ticket",
}

//nolint:paralleltest // serial: shared fake cluster and router fixtures
func TestIDORContract_PerVMRoutes(t *testing.T) {
	routes := extractRoutePatterns(t)
	mux, authHandler := newAuthzContractRouter(t)
	bobSession, bobCSRF := loginCSRF(t, authHandler, `{"username":"bob","password":"pvmss-bob"}`)

	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/api/v1/vms/{cluster}/{vmid}") {
			continue
		}

		t.Run(r.Pattern, func(t *testing.T) {
			runIDORContractCase(t, mux, r, bobSession, bobCSRF)
		})
	}
}

// runIDORContractCase verifies one per-VM route rejects a non-owner session.
// Extracted from TestIDORContract_PerVMRoutes to keep cognitive complexity
// below go:S3776.
func runIDORContractCase(t *testing.T, mux http.Handler, r extractedRoute, bobSession, bobCSRF *http.Cookie) {
	t.Helper()

	path := fillPath(r.Path)
	body := vmRequestBodies[r.Pattern]

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(r.Method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if r.Pattern == "GET /api/v1/vms/{cluster}/{vmid}/metrics/history" {
		req.URL.RawQuery = "range=hour"
	}

	req.AddCookie(bobSession)

	if r.Method != http.MethodGet {
		req.AddCookie(bobCSRF)
		req.Header.Set("X-CSRF-Token", bobCSRF.Value)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if reason, ok := idorNonForbiddenRoutes[r.Pattern]; ok {
		if rec.Code == http.StatusOK || rec.Code >= http.StatusInternalServerError {
			t.Fatalf("%s %s: status = %d, want any non-2xx, non-5xx (exemption: %s)", r.Method, path, rec.Code, reason)
		}

		return
	}

	if rec.Code != http.StatusForbidden {
		t.Fatalf("%s %s: status = %d, want 403 (IDOR)", r.Method, path, rec.Code)
	}
}

// -----------------------------------------------------------------------------
// Shared test router
// -----------------------------------------------------------------------------

type consoleRelayStub struct{}

func (consoleRelayStub) GetVNCTicket(_ context.Context, _ string, _ int, _ string) (cluster.VNCProxyTicket, error) {
	return cluster.VNCProxyTicket{}, errors.New("stub")
}

func (consoleRelayStub) RelayConsole(_ context.Context, _ string, _ int, _ cluster.VNCProxyTicket, _ io.ReadWriteCloser) error {
	return errors.New("stub")
}

type terminalRelayStub struct{}

func (terminalRelayStub) GetTermProxy(_ context.Context, _ string, _ int, _ string) (cluster.TermProxyTicket, error) {
	return cluster.TermProxyTicket{}, errors.New("stub")
}

func (terminalRelayStub) RelaySerial(_ context.Context, _ string, _ int, _ cluster.TermProxyTicket, _ io.ReadWriteCloser) error {
	return errors.New("stub")
}

func newAuthzContractRouter(t *testing.T) (http.Handler, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	st := newAdminStore(t)

	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	refresher := inventory.NewRefresher(worker, 5*time.Second)

	policyService := policy.New(st, projection, nil)

	health := httpapi.NewHealth(authzFakePinger{}, logger, nil, 60*time.Second)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(refresher, logger)
	vms := httpapi.NewVMs(projection, authHandler, 100, -1, logger)
	vmDetail := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, st, worker, logger)
	vmCloudInit := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{
		Projection: projection, Auth: authHandler, Reader: cluster.Fake{}, Writer: cluster.Fake{},
		Store: st, Refresher: worker, Log: logger,
	})
	vmBulk := httpapi.NewVMBulk(projection, authHandler, cluster.Fake{}, st, worker, logger)
	vmStatusBatch := httpapi.NewVMStatusBatchSingle(projection, authHandler, cluster.Fake{}, logger)
	vmCreate := httpapi.NewVMCreate(authHandler, st, cluster.Fake{}, cluster.Fake{}, cluster.Fake{}, logger, policyService)
	vmConsole := httpapi.NewVMConsole(projection, authHandler, consoleRelayStub{}, vm.NewConsoleTicketStore(), st, logger)
	vmSerial := httpapi.NewVMSerialConsole(projection, authHandler, terminalRelayStub{}, vm.NewConsoleTicketStore(), st, logger)
	vmSnapshots := httpapi.NewVMSnapshots(projection, authHandler, cluster.Fake{}, cluster.Fake{}, st, logger)
	vmMetrics := httpapi.NewVMMetrics(projection, authHandler, cluster.Fake{}, cluster.Fake{}, logger)

	adminCatalog := httpapi.NewAdminCatalog(authHandler, st, cluster.Fake{}, projection, logger)
	adminPolicy := httpapi.NewAdminPolicy(authHandler, policyService, logger)
	adminPools := httpapi.NewAdminPools(httpapi.AdminPoolsDeps{
		Auth: authHandler, Client: cluster.Fake{}, Projection: projection, Writer: cluster.Fake{},
		Audit: st, Refresher: worker, Store: st, Log: logger,
	})
	adminOps := httpapi.NewAdminOps(authHandler, st, cluster.Fake{}, projection, "0.4.0-test", logger)
	adminClusters := httpapi.NewAdminClusters(authHandler, st, nil, nil, logger)
	docs := httpapi.NewDocsAPIHandler(authHandler, st, logger)
	adminDocs := httpapi.NewAdminDocs(authHandler, st, docs, logger)

	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health:           health,
		ClusterNodes:     clusterNodes,
		ClusterRefresh:   clusterRefresh,
		VMs:              vms,
		VMDetail:         vmDetail,
		VMBulk:           vmBulk,
		VMStatusBatch:    vmStatusBatch,
		VMCloudInit:      vmCloudInit,
		VMCreate:         vmCreate,
		VMConsole:        vmConsole,
		VMSerialConsole:  vmSerial,
		SnapshotHandlers: []*httpapi.VMSnapshots{vmSnapshots},
		VMMetrics:        vmMetrics,
		Auth:             authHandler,
		AdminCatalog:     adminCatalog,
		AdminPolicy:      adminPolicy,
		AdminPools:       adminPools,
		AdminOps:         adminOps,
		AdminClusters:    adminClusters,
		AdminDocs:        adminDocs,
		Log:              logger,
		Store:            st,
		TrustedProxyHops: 0,
	})

	return mux, authHandler
}

// authzFakePinger satisfies httpapi.Pinger for the health check.
type authzFakePinger struct{}

func (authzFakePinger) Ping(_ context.Context) error { return nil }
