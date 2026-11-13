//nolint:noctx,paralleltest,wsl_v5 // HTTP tests use shared fake and session fixtures
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"strings"
	"testing"
	"time"
)

const (
	recoveryOldTag  = "oldtag"
	recoveryOldPool = "pvmss-old"
)

type recoveryVMRefDTO struct {
	VMID   int      `json:"vmid"`
	Name   string   `json:"name"`
	Node   string   `json:"node"`
	Status string   `json:"status"`
	Pool   string   `json:"pool"`
	Tags   []string `json:"tags"`
}

type recoveryPoolDTO struct {
	Name    string             `json:"name"`
	Comment string             `json:"comment"`
	Members []recoveryVMRefDTO `json:"members"`
}

type recoveryTagDTO struct {
	Name    string `json:"name"`
	VMCount int    `json:"vmCount"`
	Color   string `json:"color"`
}

type recoveryReportDTO struct {
	Cluster   string             `json:"cluster"`
	ScannedAt string             `json:"scannedAt"`
	Pools     []recoveryPoolDTO  `json:"pools"`
	LooseVMs  []recoveryVMRefDTO `json:"looseVMs"`
	Tags      []recoveryTagDTO   `json:"tags"`
}

type recoveryAdoptResultDTO struct {
	Pools []string         `json:"pools"`
	Tags  []recoveryTagDTO `json:"tags"`
}

// newAdminRecoveryHandler builds the recovery handler against the fake client
// plus a custom inventory index that simulates leftovers from a previous
// install: a pvmss-old pool with member VMs, a loose pvmss-tagged VM, and
// tags unknown to the fresh catalog.
func newAdminRecoveryHandler(t *testing.T, cooldown ...time.Duration) (*httpapi.AdminRecovery, *httpapi.Auth, *inventory.Projection, cluster.Fake) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	client := cluster.Fake{}
	if err := client.CreatePool(context.Background(), recoveryOldPool, "previous install"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	index := inventory.BuildIndex(cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 900, Name: "web-old-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: recoveryOldPool, Tags: []string{extraPvmssTag, recoveryOldTag}},
		{VMID: 901, Name: "web-old-02", Node: cluster.FakeNode01, Status: cluster.VMStopped, Pool: recoveryOldPool, Tags: []string{extraPvmssTag}},
		{VMID: 902, Name: "solo-old", Node: cluster.FakeNode02, Status: cluster.VMStopped, Tags: []string{extraPvmssTag, recoveryOldTag}},
		{VMID: 903, Name: "foreign", Node: cluster.FakeNode02, Status: cluster.VMRunning, Pool: cluster.FakePoolShared},
	}})
	projection := inventory.NewProjectionFromIndex(&index)
	st := newAdminStore(t)
	provider := vmCreateClientProvider{clients: map[string]cluster.Client{auditTestCluster: client}}
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{auditTestCluster: &index})
	deps := httpapi.AdminRecoveryDeps{Auth: authHandler, Clients: provider, Source: registry, Projection: projection, Store: st, Log: slog.Default()}
	if len(cooldown) > 0 {
		deps.ScanCooldown = cooldown[0]
	}
	handler := httpapi.NewAdminRecovery(deps)
	return handler, authHandler, projection, client
}

func adminRecoveryRequest(t *testing.T, handler http.HandlerFunc, method, path string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

// getRecoveryReport serves GET /admin/recovery on the default cluster and
// decodes the report payload.
func getRecoveryReport(t *testing.T, handler *httpapi.AdminRecovery, cookie *http.Cookie) recoveryReportDTO {
	t.Helper()
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return report
}

func TestAdminRecovery_ReportListsOrphans(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	report := getRecoveryReport(t, handler, adminCookie(t, authHandler))
	if len(report.Pools) != 1 || report.Pools[0].Name != recoveryOldPool || len(report.Pools[0].Members) != 2 {
		t.Fatalf("pools = %+v, want pvmss-old with 2 members", report.Pools)
	}
	if report.Pools[0].Members[0].Name != "web-old-01" {
		t.Fatalf("member = %+v", report.Pools[0].Members[0])
	}
	if len(report.LooseVMs) != 1 || report.LooseVMs[0].VMID != 902 {
		t.Fatalf("looseVMs = %+v, want only 902", report.LooseVMs)
	}
	if len(report.Tags) != 1 || report.Tags[0].Name != recoveryOldTag || report.Tags[0].VMCount != 2 || report.Tags[0].Color == "" {
		t.Fatalf("tags = %+v, want oldtag x2 with suggested color", report.Tags)
	}
	if report.ScannedAt == "" {
		t.Fatal("scannedAt missing from report")
	}
}

func TestAdminRecovery_ReportForbiddenForNonAdmin(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", aliceCookie(t, authHandler), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAdminRecovery_AdoptRegistersPoolsAndTags(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminRecoveryRequest(t, handler.ServeAdopt, http.MethodPost, "/api/v1/admin/recovery", cookie,
		`{"cluster":"default","pools":["pvmss-old"],"tags":["oldtag"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("adopt status = %d: %s", rec.Code, rec.Body.String())
	}
	var result recoveryAdoptResultDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Pools) != 1 || result.Pools[0] != recoveryOldPool || len(result.Tags) != 1 || result.Tags[0].Name != recoveryOldTag {
		t.Fatalf("result = %+v", result)
	}

	report := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var after recoveryReportDTO
	if err := json.Unmarshal(report.Body.Bytes(), &after); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(after.Pools) != 0 || len(after.Tags) != 0 {
		t.Fatalf("report after adopt = %+v, want no orphans left", after)
	}
	if len(after.LooseVMs) != 1 {
		t.Fatalf("looseVMs after adopt = %+v, want the pool-less VM still listed", after.LooseVMs)
	}
}

// Repeat GETs within the cooldown serve the cached report: the scan hits
// Proxmox, so page reloads must not each trigger a cluster call.
func TestAdminRecovery_ReportCachedWithinCooldown(t *testing.T) {
	handler, authHandler, _, fakeClient := newAdminRecoveryHandler(t)
	cookie := adminCookie(t, authHandler)

	if rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("first scan status = %d: %s", rec.Code, rec.Body.String())
	}
	if err := fakeClient.DeletePool(context.Background(), recoveryOldPool); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Pools) != 1 || report.Pools[0].Name != recoveryOldPool {
		t.Fatalf("cached report pools = %+v, want pvmss-old still served", report.Pools)
	}
}

// POST /scan is the explicit "scan again" action: it bypasses the cooldown
// and refreshes the cache subsequent GETs read.
func TestAdminRecovery_ScanForcesRefresh(t *testing.T) {
	handler, authHandler, _, fakeClient := newAdminRecoveryHandler(t)
	cookie := adminCookie(t, authHandler)

	adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	if err := fakeClient.DeletePool(context.Background(), recoveryOldPool); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}
	rec := adminRecoveryRequest(t, handler.ServeScan, http.MethodPost, "/api/v1/admin/recovery/scan", cookie, `{"cluster":"default"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("scan status = %d: %s", rec.Code, rec.Body.String())
	}
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Pools) != 0 {
		t.Fatalf("forced scan pools = %+v, want empty after pool deletion", report.Pools)
	}
	again := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var refreshed recoveryReportDTO
	if err := json.Unmarshal(again.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(refreshed.Pools) != 0 {
		t.Fatalf("cached report after forced scan = %+v, want empty", refreshed.Pools)
	}
}

// Once nothing is adoptable the feature goes quiet: a leftover pool created
// afterwards does not trigger an automatic rescan - only POST /scan does.
func TestAdminRecovery_EmptyReportIsNotRescanned(t *testing.T) {
	handler, authHandler, _, fakeClient := newAdminRecoveryHandler(t)
	cookie := adminCookie(t, authHandler)

	adopt := adminRecoveryRequest(t, handler.ServeAdopt, http.MethodPost, "/api/v1/admin/recovery", cookie,
		`{"cluster":"default","pools":["pvmss-old"],"tags":["oldtag"]}`)
	if adopt.Code != http.StatusOK {
		t.Fatalf("adopt status = %d: %s", adopt.Code, adopt.Body.String())
	}
	empty := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var drained recoveryReportDTO
	if err := json.Unmarshal(empty.Body.Bytes(), &drained); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(drained.Pools) != 0 || len(drained.Tags) != 0 {
		t.Fatalf("report after adopt = %+v, want nothing adoptable", drained)
	}
	if err := fakeClient.CreatePool(context.Background(), "pvmss-late", "second install"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Pools) != 0 {
		t.Fatalf("pinned report pools = %+v, want empty (no automatic rescan)", report.Pools)
	}
	forced := adminRecoveryRequest(t, handler.ServeScan, http.MethodPost, "/api/v1/admin/recovery/scan", cookie, `{"cluster":"default"}`)
	var forcedReport recoveryReportDTO
	if err := json.Unmarshal(forced.Body.Bytes(), &forcedReport); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(forcedReport.Pools) != 1 || forcedReport.Pools[0].Name != "pvmss-late" {
		t.Fatalf("forced scan pools = %+v, want pvmss-late", forcedReport.Pools)
	}
}

// A report that still contains adoptable orphans expires after the cooldown,
// so a revisit later that day re-scans instead of serving stale data.
func TestAdminRecovery_ExpiredCooldownRescans(t *testing.T) {
	handler, authHandler, _, fakeClient := newAdminRecoveryHandler(t, time.Nanosecond)
	cookie := adminCookie(t, authHandler)

	adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	if err := fakeClient.DeletePool(context.Background(), recoveryOldPool); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Pools) != 0 {
		t.Fatalf("report after cooldown = %+v, want rescan showing no pools", report.Pools)
	}
}

// newRecoveryDeps builds the default handler deps so error-path tests can
// swap one collaborator at a time (no source registry, empty client list,
// closed store, unpopulated projection).
func newRecoveryDeps(t *testing.T) (httpapi.AdminRecoveryDeps, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	client := cluster.Fake{}
	if err := client.CreatePool(context.Background(), recoveryOldPool, "previous install"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	index := inventory.BuildIndex(cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 900, Name: "web-old-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: recoveryOldPool, Tags: []string{extraPvmssTag}},
	}})

	return httpapi.AdminRecoveryDeps{
		Auth:       authHandler,
		Clients:    vmCreateClientProvider{clients: map[string]cluster.Client{auditTestCluster: client}},
		Projection: inventory.NewProjectionFromIndex(&index),
		Store:      newAdminStore(t),
		Log:        slog.Default(),
	}, authHandler
}

func TestAdminRecovery_ReportUnknownCluster404(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=bogus", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAdminRecovery_ReportFallsBackToFirstCluster(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.Cluster != auditTestCluster {
		t.Fatalf("cluster = %q, want %q", report.Cluster, auditTestCluster)
	}
}

func TestAdminRecovery_ReportUnauthenticated(t *testing.T) {
	handler, _, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// An unpopulated projection means the inventory has not refreshed yet - the
// report is a 503, not a failure.
func TestAdminRecovery_ReportProjectionNotReady503(t *testing.T) {
	deps, authHandler := newRecoveryDeps(t)
	deps.Projection = inventory.NewProjection()
	handler := httpapi.NewAdminRecovery(deps)

	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
}

// A dead store mid-scan is the generic 500 path of writeRecoveryError.
func TestAdminRecovery_ReportStoreFailure500(t *testing.T) {
	deps, authHandler := newRecoveryDeps(t)
	if err := deps.Store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	handler := httpapi.NewAdminRecovery(deps)

	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRecovery_ReportNilClients404(t *testing.T) {
	deps, authHandler := newRecoveryDeps(t)
	deps.Clients = nil
	handler := httpapi.NewAdminRecovery(deps)

	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAdminRecovery_ReportEmptyClientList404(t *testing.T) {
	deps, authHandler := newRecoveryDeps(t)
	deps.Clients = vmCreateClientProvider{clients: map[string]cluster.Client{}}
	handler := httpapi.NewAdminRecovery(deps)

	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAdminRecovery_ScanBadJSON(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeScan, http.MethodPost, "/api/v1/admin/recovery/scan", adminCookie(t, authHandler), "{invalid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// With several clusters registered an empty cluster field is ambiguous.
func TestAdminRecovery_ScanClusterRequired(t *testing.T) {
	deps, authHandler := newRecoveryDeps(t)
	deps.Clients = vmCreateClientProvider{clients: map[string]cluster.Client{"a": cluster.Fake{}, "b": cluster.Fake{}}}
	handler := httpapi.NewAdminRecovery(deps)

	rec := adminRecoveryRequest(t, handler.ServeScan, http.MethodPost, "/api/v1/admin/recovery/scan", adminCookie(t, authHandler), `{"cluster":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAdminRecovery_AdoptBadJSON(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeAdopt, http.MethodPost, "/api/v1/admin/recovery", adminCookie(t, authHandler), "{invalid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAdminRecovery_AdoptClusterRequired(t *testing.T) {
	deps, authHandler := newRecoveryDeps(t)
	deps.Clients = vmCreateClientProvider{clients: map[string]cluster.Client{"a": cluster.Fake{}, "b": cluster.Fake{}}}
	handler := httpapi.NewAdminRecovery(deps)

	rec := adminRecoveryRequest(t, handler.ServeAdopt, http.MethodPost, "/api/v1/admin/recovery", adminCookie(t, authHandler), `{"cluster":"","pools":["pvmss-old"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// A named-but-unknown cluster passes ResolveClusterValue and dies in
// adoptionDeps instead.
func TestAdminRecovery_AdoptUnknownCluster404(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeAdopt, http.MethodPost, "/api/v1/admin/recovery", adminCookie(t, authHandler), `{"cluster":"bogus","pools":["pvmss-old"]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// Adopt through a proxy-hops-configured handler still audits with the right
// client IP source.
func TestAdminRecovery_AdoptWithTrustedProxyHops(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	handler.SetTrustedProxyHops(1)
	cookie := adminCookie(t, authHandler)

	rec := adminRecoveryRequest(t, handler.ServeAdopt, http.MethodPost, "/api/v1/admin/recovery", cookie,
		`{"cluster":"default","pools":["pvmss-old"],"tags":["oldtag"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("adopt status = %d: %s", rec.Code, rec.Body.String())
	}
}
