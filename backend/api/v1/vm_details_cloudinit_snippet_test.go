package apiv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"pvmss/proxmox"
	"pvmss/state"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

<<<<<<< HEAD
=======
// --- redactCloudInitSecrets unit tests ---

func TestRedactCloudInitSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{
			name: "redacts password hash, keeps key and indentation",
			in:   "user: jhmt\npassword: $5$abc$def\nchpasswd:\n  expire: False\n",
			want: "user: jhmt\npassword: <redacted>\nchpasswd:\n  expire: False\n",
		},
		{
			name: "redacts hashed_passwd and plain_text_passwd",
			in:   "users:\n  - name: a\n    hashed_passwd: $6$xx\n    plain_text_passwd: secret\n",
			want: "users:\n  - name: a\n    hashed_passwd: <redacted>\n    plain_text_passwd: <redacted>\n",
		},
		{
			name: "leaves non-secret lines untouched",
			in:   "#cloud-config\npackages:\n  - curl\nruncmd:\n  - echo password: not-a-key\n",
			want: "#cloud-config\npackages:\n  - curl\nruncmd:\n  - echo password: not-a-key\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, redactCloudInitSecrets(tt.in))
		})
	}
}

>>>>>>> 8902630 (feat(cloud-init): editable cloud-init + per-VM custom YAML with read-only fallback)
// --- parseCICustomUser unit tests (table-driven) ---

func TestParseCICustomUser(t *testing.T) {
	tests := []struct {
		name         string
		cicustom     string
		wantStorage  string
		wantFilename string
	}{
		{
			name:         "user entry only",
			cicustom:     "user=local:snippets/pvmss-100.yml",
			wantStorage:  "local",
			wantFilename: "pvmss-100.yml",
		},
		{
			name:         "user entry among others",
			cicustom:     "vendor=local:snippets/vendor.yml,user=local:snippets/pvmss-100.yml",
			wantStorage:  "local",
			wantFilename: "pvmss-100.yml",
		},
		{
			name:         "no user entry returns empty",
			cicustom:     "vendor=local:snippets/vendor.yml",
			wantStorage:  "",
			wantFilename: "",
		},
		{
			name:         "empty input returns empty",
			cicustom:     "",
			wantStorage:  "",
			wantFilename: "",
		},
		{
			name:         "user entry with surrounding whitespace is trimmed",
			cicustom:     "  user=local:snippets/pvmss-42.yml  ",
			wantStorage:  "local",
			wantFilename: "pvmss-42.yml",
		},
		{
			name:         "malformed user entry (no colon) returns empty",
			cicustom:     "user=local",
			wantStorage:  "",
			wantFilename: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, filename := parseCICustomUser(tt.cicustom)
			assert.Equal(t, tt.wantStorage, storage)
			assert.Equal(t, tt.wantFilename, filename)
		})
	}
}

// --- snippet handler validation-rejection tests (offline gate) ---
//
// These reuse the ciStubState from vm_cloudinit_test.go. The handlers gate on
// SFTP being configured before the offline check, so we use a stub that
// exposes settings with SFTP enabled.

// snippetStubState extends ciStubState to expose SFTP-enabled settings so the
// snippet handlers reach the offline gate (where validation has already run).
type snippetStubState struct {
	ciStubState
	sftpEnabled bool
}

// GetSettings returns settings with SFTP enabled/disabled per the stub config.
// Only the CloudInitSFTP.Enabled flag matters for the snippet handlers' gate.
func (s *snippetStubState) GetSettings() *state.AppSettings {
	return &state.AppSettings{
		CloudInitSFTP: proxmox.CloudInitSFTPConfig{Enabled: s.sftpEnabled},
	}
}

// newSnippetStubState returns a stub with SFTP enabled by default so the
// handlers' SFTP gate passes and validation tests can proceed to the offline
// check (503).
func newSnippetStubState(sftpEnabled, offline bool) *snippetStubState {
	return &snippetStubState{ciStubState: ciStubState{offline: offline}, sftpEnabled: sftpEnabled}
}

func newSnippetRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	// httptest.NewRequest does not run the router, so populate the :id route
	// param that requireVMID expects. "100" is used in the path above.
	ctx := context.WithValue(r.Context(), httprouter.ParamsKey, httprouter.Params{
		{Key: "id", Value: "100"},
	})
	return r.WithContext(ctx)
}

func TestGetVMCloudInitSnippet_SFTPDisabledIsAllowed(t *testing.T) {
	// GET is permitted without SFTP: it falls back to a read-only dump. With
	// SFTP disabled the handler must NOT short-circuit with sftp_disabled; it
	// proceeds and (in offline mode) reaches the offline gate (503).
	h := MakeVMDetailsHandler(newSnippetStubState(false, true))
	w := httptest.NewRecorder()
	h.GetVMCloudInitSnippet(w, newSnippetRequest(http.MethodGet, "/api/v1/vms/100/cloudinit/snippet", ""))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.NotContains(t, w.Body.String(), "sftp_disabled")
}

func TestUpdateVMCloudInitSnippet_SFTPDisabledReturns400(t *testing.T) {
	h := MakeVMDetailsHandler(newSnippetStubState(false, true))
	w := httptest.NewRecorder()
	h.UpdateVMCloudInitSnippet(w, newSnippetRequest(http.MethodPut, "/api/v1/vms/100/cloudinit/snippet", `{"content":"#cloud-config\n"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "sftp_disabled")
}

func TestUpdateVMCloudInitSnippet_RejectsInvalidYAML(t *testing.T) {
	h := MakeVMDetailsHandler(newSnippetStubState(true, true))
	w := httptest.NewRecorder()
	// Missing #cloud-config header — strict validator rejects.
	h.UpdateVMCloudInitSnippet(w, newSnippetRequest(http.MethodPut, "/api/v1/vms/100/cloudinit/snippet", `{"content":"package_update: true"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInitSnippet_RejectsMalformedYAML(t *testing.T) {
	h := MakeVMDetailsHandler(newSnippetStubState(true, true))
	w := httptest.NewRecorder()
	// Unclosed mapping — yaml.Unmarshal fails.
	h.UpdateVMCloudInitSnippet(w, newSnippetRequest(http.MethodPut, "/api/v1/vms/100/cloudinit/snippet", `{"content":"#cloud-config\npackages:\n  - [oops"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInitSnippet_RejectsInvalidJSON(t *testing.T) {
	h := MakeVMDetailsHandler(newSnippetStubState(true, true))
	w := httptest.NewRecorder()
	h.UpdateVMCloudInitSnippet(w, newSnippetRequest(http.MethodPut, "/api/v1/vms/100/cloudinit/snippet", "not json"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateVMCloudInitSnippet_ValidInputReachesOfflineGate(t *testing.T) {
	h := MakeVMDetailsHandler(newSnippetStubState(true, true))
	w := httptest.NewRecorder()
	h.UpdateVMCloudInitSnippet(w, newSnippetRequest(http.MethodPut, "/api/v1/vms/100/cloudinit/snippet", `{"content":"#cloud-config\npackage_update: true\n"}`))
	// Validation passed; SFTP enabled; now the offline gate fires (503).
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetVMCloudInitSnippet_ValidInputReachesOfflineGate(t *testing.T) {
	h := MakeVMDetailsHandler(newSnippetStubState(true, true))
	w := httptest.NewRecorder()
	h.GetVMCloudInitSnippet(w, newSnippetRequest(http.MethodGet, "/api/v1/vms/100/cloudinit/snippet", ""))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
