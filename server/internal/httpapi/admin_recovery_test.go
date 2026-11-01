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
	if err := client.CreatePool(context.Background(), "pvmss-old", "previous install"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	index := inventory.BuildIndex(cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 900, Name: "web-old-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: "pvmss-old", Tags: []string{"pvmss", "oldtag"}},
		{VMID: 901, Name: "web-old-02", Node: cluster.FakeNode01, Status: cluster.VMStopped, Pool: "pvmss-old", Tags: []string{"pvmss"}},
		{VMID: 902, Name: "solo-old", Node: cluster.FakeNode02, Status: cluster.VMStopped, Tags: []string{"pvmss", "oldtag"}},
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

func TestAdminRecovery_ReportListsOrphans(t *testing.T) {
	handler, authHandler, _, _ := newAdminRecoveryHandler(t)
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", adminCookie(t, authHandler), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Pools) != 1 || report.Pools[0].Name != "pvmss-old" || len(report.Pools[0].Members) != 2 {
		t.Fatalf("pools = %+v, want pvmss-old with 2 members", report.Pools)
	}
	if report.Pools[0].Members[0].Name != "web-old-01" {
		t.Fatalf("member = %+v", report.Pools[0].Members[0])
	}
	if len(report.LooseVMs) != 1 || report.LooseVMs[0].VMID != 902 {
		t.Fatalf("looseVMs = %+v, want only 902", report.LooseVMs)
	}
	if len(report.Tags) != 1 || report.Tags[0].Name != "oldtag" || report.Tags[0].VMCount != 2 || report.Tags[0].Color == "" {
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
	if len(result.Pools) != 1 || result.Pools[0] != "pvmss-old" || len(result.Tags) != 1 || result.Tags[0].Name != "oldtag" {
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
	if err := fakeClient.DeletePool(context.Background(), "pvmss-old"); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}
	rec := adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	var report recoveryReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Pools) != 1 || report.Pools[0].Name != "pvmss-old" {
		t.Fatalf("cached report pools = %+v, want pvmss-old still served", report.Pools)
	}
}

// POST /scan is the explicit "scan again" action: it bypasses the cooldown
// and refreshes the cache subsequent GETs read.
func TestAdminRecovery_ScanForcesRefresh(t *testing.T) {
	handler, authHandler, _, fakeClient := newAdminRecoveryHandler(t)
	cookie := adminCookie(t, authHandler)

	adminRecoveryRequest(t, handler.ServeReport, http.MethodGet, "/api/v1/admin/recovery?cluster=default", cookie, "")
	if err := fakeClient.DeletePool(context.Background(), "pvmss-old"); err != nil {
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
	if err := fakeClient.DeletePool(context.Background(), "pvmss-old"); err != nil {
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
