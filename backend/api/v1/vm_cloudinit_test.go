package apiv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	envpkg "pvmss/env"
	"pvmss/state"
)

// --- Validator unit tests (table-driven) ---

func TestValidateCIUser(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		wantErr bool
	}{
		{"valid simple", "ubuntu", false},
		{"valid with dot/hyphen/underscore", "my.user-1_admin", false},
		{"empty rejected", "", true},
		{"too long", strings.Repeat("a", 33), true},
		{"invalid char", "root!", true},
		{"space rejected", "ro ot", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCIUser(tt.user)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCIPassword(t *testing.T) {
	tests := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{"valid", "S3cr3t!", false},
		{"empty rejected", "", true},
		{"max length", strings.Repeat("a", 256), false},
		{"too long", strings.Repeat("a", 257), true},
		{"control char rejected", "pass\x00word", true},
		{"newline rejected", "pass\nword", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCIPassword(tt.pwd)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCISSHKeys(t *testing.T) {
	validRSA := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQctest user@host"
	validEd := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest"
	tests := []struct {
		name    string
		keys    string
		wantErr bool
	}{
		{"valid single", validRSA, false},
		{"valid multiple", validRSA + "\n" + validEd, false},
		{"blank lines ignored", "\n" + validRSA + "\n\n", false},
		{"empty rejected", "", true},
		{"whitespace only rejected", "   \n  ", true},
		{"invalid prefix", "not-a-key AAAA", true},
		{"missing space", "ssh-rsa", true},
		{"too many keys", strings.Repeat(validRSA+"\n", 41), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCISSHKeys(tt.keys)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCIIPConfig(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"dhcp", "ip=dhcp", false},
		{"static with gw", "ip=192.168.1.50/24,gw=192.168.1.1", false},
		{"ipv6 auto", "ip6=auto", false},
		{"ipv6 static", "ip6=fd00::1/64,gw6=fd00::ffff", false},
		{"empty rejected", "", true},
		{"bad token", "ip=dhcp,bogus", true},
		{"bad cidr", "ip=192.168.1.50", true},
		{"bad gw", "ip=192.168.1.50/24,gw=notanip", true},
		{"unknown key", "foo=bar", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCIIPConfig(tt.ip)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCIDNSList(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"single ipv4", "8.8.8.8", false},
		{"multiple", "8.8.8.8,1.1.1.1", false},
		{"ipv6", "2001:4860:4860::8888", false},
		{"trailing comma ignored", "8.8.8.8,", false},
		{"empty rejected", "", true},
		{"not an ip", "example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCIDNSList(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCISearchdomain(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"single", "example.com", false},
		{"multiple", "example.com,sub.example.org", false},
		{"trailing comma ignored", "example.com,", false},
		{"empty rejected", "", true},
		{"invalid label", "exa_mple.com", true},
		{"label too long", strings.Repeat("a", 64) + ".com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCISearchdomain(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- Handler validation-rejection tests ---

// ciStubState is a minimal state.StateManager stub for the cloud-init handler.
// Only the methods reached before the Proxmox call are overridden.
type ciStubState struct {
	state.StateManager
	offline bool
}

func (s *ciStubState) IsOfflineMode() bool { return s.offline }
func (s *ciStubState) GetEnvConfig() *envpkg.EnvConfig {
	return &envpkg.EnvConfig{Offline: s.offline}
}

func newCloudInitRequest(vmid string, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/api/v1/vms/"+vmid+"/cloudinit", strings.NewReader(body))
	ctx := context.WithValue(r.Context(), httprouter.ParamsKey, httprouter.Params{
		{Key: "id", Value: vmid},
	})
	return r.WithContext(ctx)
}

func TestUpdateVMCloudInit_RejectsInvalidJSON(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", "not json"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInit_RejectsEmptyUpdate(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "no cloud-init fields to update")
}

func TestUpdateVMCloudInit_RejectsBadUser(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{"user":"bad user!"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInit_RejectsBadSSHKeys(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{"ssh_keys":"not-a-key"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInit_RejectsBadIPConfig(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{"ip_config":"ip=notacidr"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInit_RejectsBadNameserver(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{"nameserver":"example.com"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInit_RejectsBadSearchdomain(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{"searchdomain":"exa_mple.com"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInit_RejectsBadPassword(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", `{"password":"`+strings.Repeat("a", 257)+`"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateVMCloudInit_ValidInputReachesOfflineGate confirms that a well-
// formed request passes validation and progresses to the offline check (503)
// without needing a live Proxmox connection. This exercises the full validation
// happy path and the pool-membership/Proxmox wiring is not reached.
func TestUpdateVMCloudInit_ValidInputReachesOfflineGate(t *testing.T) {
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	body := `{"user":"ubuntu","password":"S3cr3t!","ssh_keys":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest","ip_config":"ip=192.168.1.50/24,gw=192.168.1.1","nameserver":"8.8.8.8","searchdomain":"example.com"}`
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", body))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUpdateVMCloudInit_AllowsClearingFields(t *testing.T) {
	// Non-nil empty strings mean "clear" and must pass validation (they skip
	// the field validators). Combined with a valid ip_config so the update is
	// non-empty; offline gate then returns 503.
	h := MakeVMDetailsHandler(&ciStubState{offline: true})
	w := httptest.NewRecorder()
	body := `{"user":"","ssh_keys":"","nameserver":"","searchdomain":"","ip_config":"ip=dhcp"}`
	h.UpdateVMCloudInit(w, newCloudInitRequest("100", body))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
