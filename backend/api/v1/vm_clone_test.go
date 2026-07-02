package apiv1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
)

// cloneRequest builds a POST clone request with the :id param in context and a JSON body.
func cloneRequest(vmid, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/vms/"+vmid+"/clone", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(r.Context(), httprouter.ParamsKey, httprouter.Params{{Key: "id", Value: vmid}})
	return r.WithContext(ctx)
}

// TestCloneVM_InvalidName characterizes name validation, which runs before the
// offline gate so it is exercisable without Proxmox.
func TestCloneVM_InvalidName(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	for _, body := range []string{`{"name":""}`, `{"name":"bad name"}`, `{"name":"-lead"}`} {
		w := httptest.NewRecorder()
		h.CloneVM(w, cloneRequest("101", body))
		assert.Equal(t, http.StatusBadRequest, w.Code, "body %s should be rejected", body)
	}
}

// TestCloneVM_OfflineGate confirms a valid request is refused with 503 in offline mode.
func TestCloneVM_OfflineGate(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	w := httptest.NewRecorder()
	h.CloneVM(w, cloneRequest("101", `{"name":"web-clone-01"}`))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
