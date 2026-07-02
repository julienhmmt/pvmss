package apiv1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
)

// TestListPools_Offline_ReturnsEmptyArray locks the offline contract: ListPools
// short-circuits to an empty (non-nil) JSON array before touching Proxmox, so
// the concurrent-fetch refactor never runs offline. The concurrent fan-out
// itself is exercised by the race detector on the live/integration path (no
// client-injection seam exists to mock it offline).
func TestListPools_Offline_ReturnsEmptyArray(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	w := httptest.NewRecorder()
	h.ListPools(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/userpool", nil))

	require.Equal(t, http.StatusOK, w.Code)

	var resp []apiv1.AdminPoolResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp, "must be [] not null")
	assert.Empty(t, resp)
}
