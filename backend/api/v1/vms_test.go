package apiv1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pvmss/proxmox"
)

func TestListVMs_OfflineMode(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := MakeVMHandler(sm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})
	rr := httptest.NewRecorder()

	JWTMiddleware(sm, http.HandlerFunc(h.ListVMs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp VMListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 0 || len(resp.VMs) != 0 {
		t.Errorf("expected empty list in offline mode, got %d VMs", resp.Total)
	}
}

func TestGetVM_BadID(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	h := MakeVMHandler(sm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/notanumber", nil)
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})
	rr := httptest.NewRecorder()

	JWTMiddleware(sm, http.HandlerFunc(h.GetVM)).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestVmToSummary_ByteConversion(t *testing.T) {
	const gb = int64(1024 * 1024 * 1024)
	vm := proxmox.VM{
		VMID:    100,
		Name:    "test-vm",
		Node:    "pve",
		Status:  "running",
		MaxDisk: 10 * gb,
		MaxMem:  4 * gb,
		Mem:     2 * gb,
	}
	s := vmToSummary(vm)

	if s.DiskMB != 10*1024 {
		t.Errorf("DiskMB: expected 10240, got %d", s.DiskMB)
	}
	if s.MaxMemMB != 4*1024 {
		t.Errorf("MaxMemMB: expected 4096, got %d", s.MaxMemMB)
	}
	if s.MemMB != 2*1024 {
		t.Errorf("MemMB: expected 2048, got %d", s.MemMB)
	}
}
