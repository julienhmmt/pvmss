package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pvmss/server/internal/cluster"
)

// TestVMDetail_MutationsKeepProjectionConsistent - a rename, then a hardware
// change, then a disk add must each leave the projection showing every
// earlier write. The fixture projection was refreshed just now, so each
// post-write refresh lands inside the manual-refresh guard interval; the
// guard must not stop a post-write refresh.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_MutationsKeepProjectionConsistent(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)
	t.Cleanup(cluster.ResetFake)

	if rec, _ := serveDetail(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/101", `{"name":"b-renamed"}`, cookie)); rec.Code != http.StatusOK {
		t.Fatalf("rename status = %d; body=%s", rec.Code, rec.Body.String())
	}

	hw := httptest.NewRecorder()
	handler.ServeHTTP(hw, detailRequest(http.MethodPut, "/api/v1/vms/default/101/hardware", `{"cores":2,"memoryMB":1536}`, cookie))
	if hw.Code != http.StatusOK {
		t.Fatalf("hardware status = %d; body=%s", hw.Code, hw.Body.String())
	}

	rec, entity := serveDetail(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/101", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d; body=%s", rec.Code, rec.Body.String())
	}

	if entity.Name != "b-renamed" {
		t.Errorf("name = %q after hardware write, want b-renamed (rename lost)", entity.Name)
	}

	if entity.CPUCores != 2 {
		t.Errorf("cpuCores = %d, want 2 (cores*sockets)", entity.CPUCores)
	}

	if want := int64(1536) << 20; entity.MemoryTotal != want {
		t.Errorf("memoryTotal = %d, want %d", entity.MemoryTotal, want)
	}
}
