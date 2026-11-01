package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

// seedLeasePolicy turns leases on for the default cluster with the given
// length, keeping the shipped gabarit fields valid.
func seedLeasePolicy(t *testing.T, st *store.Store, leaseDays int) {
	t.Helper()

	row := store.PolicyRow{
		Cluster: "default", MaxSockets: 4, MaxCores: 8, MaxMemoryMB: 16384,
		MaxDiskPerVMGB: 500, MaxNetworkCards: 4, MaxSnapshots: 5, MaxVMPerUser: -1,
		LeaseDays: leaseDays,
	}
	if err := st.UpsertPolicyRow(context.Background(), row); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}
}

func seedLease(t *testing.T, st *store.Store, vmid int, expires time.Time) {
	t.Helper()

	if err := st.SetVMLease(context.Background(), store.VMLease{Cluster: "default", VMID: vmid, ExpiresAt: expires}); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}
}

// TestVMDetail_Lease_GetExposesExpiry - the detail response carries the lease
// expiry when the VM is leased.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_GetExposesExpiry(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)
	expires := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	seedLease(t, st, 100, expires)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode detail: %v", err)
	}

	if body.ExpiresAt != expires.Format(time.RFC3339) {
		t.Errorf("expiresAt = %q, want %q", body.ExpiresAt, expires.Format(time.RFC3339))
	}
}

// TestVMDetail_Lease_ExtendNonOwnerForbidden - a non-owner cannot extend.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_ExtendNonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	seedLeasePolicy(t, st, 30)
	seedLease(t, st, 100, time.Now().UTC().Add(time.Hour))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/lease", `{"days":7}`, bobCookie(t, authHandler)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_Lease_ExtendOwnerSucceeds - an owner extension returns the new
// expiry, clamped to the cluster's lease_days.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_ExtendOwnerSucceeds(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	seedLeasePolicy(t, st, 30)
	seedLease(t, st, 100, time.Now().UTC().Add(time.Hour))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/lease", `{"days":365}`, adminCookie(t, authHandler)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body leaseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode lease: %v", err)
	}

	got, err := time.Parse(time.RFC3339, body.ExpiresAt)
	if err != nil {
		t.Fatalf("parse expiresAt %q: %v", body.ExpiresAt, err)
	}

	if d := time.Until(got); d < 29*24*time.Hour || d > 30*24*time.Hour+time.Minute {
		t.Errorf("expiry in %v, want about 30 days (clamped)", d)
	}
}

type leaseBody struct {
	ExpiresAt string `json:"expiresAt"`
}

// TestVMDetail_Lease_ExtendRefusals - leases off and a VM without a lease are
// refused with their own codes.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_ExtendRefusals(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	seedLease(t, st, 100, time.Now().UTC().Add(time.Hour))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/lease", `{"days":7}`, cookie))

	if rec.Code != http.StatusConflict {
		t.Fatalf("leases off: status = %d, want 409", rec.Code)
	}

	seedLeasePolicy(t, st, 30)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/101/lease", `{"days":7}`, cookie))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("no lease row: status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_Lease_RemoveAdminOnly - only an administrator can exempt a VM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_RemoveAdminOnly(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	seedLease(t, st, 100, time.Now().UTC().Add(time.Hour))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodDelete, "/api/v1/vms/default/100/lease", "", bobCookie(t, authHandler)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: status = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodDelete, "/api/v1/vms/default/100/lease", "", adminCookie(t, authHandler)))

	if rec.Code != http.StatusOK {
		t.Fatalf("admin: status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if _, err := st.GetVMLease(context.Background(), "default", 100); err == nil {
		t.Error("lease still present after admin removal")
	}
}

// TestVMDetail_Lease_MethodNotAllowed - other methods on /lease are refused.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/lease", "", adminCookie(t, authHandler)))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_Lease_RenameKeepsLease - renaming a leased VM through PATCH
// leaves its lease in place with the same owner pool, so the expiry still
// applies to the renamed VM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Lease_RenameKeepsLease(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	expires := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	seedLease(t, st, 100, expires)

	before, err := st.GetVMLease(context.Background(), "default", 100)
	if err != nil {
		t.Fatalf("GetVMLease before: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"renamed-vm"}`, adminCookie(t, authHandler)))

	if rec.Code != http.StatusOK {
		t.Fatalf("rename status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	after, err := st.GetVMLease(context.Background(), "default", 100)
	if err != nil {
		t.Fatalf("lease dropped by rename: %v", err)
	}

	if after.Pool != before.Pool || !after.ExpiresAt.Equal(before.ExpiresAt) {
		t.Errorf("lease after rename = %+v, want unchanged %+v", after, before)
	}
}
