package apiv1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pvmss/proxmox"
)

func TestListMyVMs_OfflineMode(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := NewProfileHandler(sm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/vms", nil)
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})
	rr := httptest.NewRecorder()

	JWTMiddleware(sm, http.HandlerFunc(h.ListMyVMs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp VMListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 0 || len(resp.VMs) != 0 {
		t.Errorf("expected empty list in offline mode, got %d", resp.Total)
	}
}

func TestVMBelongsToUser(t *testing.T) {
	tests := []struct {
		tags     string
		username string
		want     bool
	}{
		{"pvmss;alice;prod", "alice", true},
		{"pvmss;alice;prod", "bob", false},
		{"", "alice", false},
		{"ALICE;pvmss", "alice", true},
		{"pvmss", "alice", false},
	}
	for _, tt := range tests {
		vm := proxmox.VM{Tags: tt.tags}
		if got := vmBelongsToUser(vm, tt.username); got != tt.want {
			t.Errorf("vmBelongsToUser(tags=%q, user=%q) = %v, want %v", tt.tags, tt.username, got, tt.want)
		}
	}
}
