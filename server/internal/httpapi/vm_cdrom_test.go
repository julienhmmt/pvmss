package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_CDROM_DistinguishesDisconnectAndRemove(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	cases := []struct {
		name      string
		body      string
		wantState string
	}{
		{name: "disconnect", body: `{"action":"disconnect"}`, wantState: cluster.CDROMEmpty},
		{name: "remove", body: `{"action":"remove"}`, wantState: cluster.CDROMAbsent},
	}
	for _, test := range cases { //nolint:paralleltest // serial: shared fake cluster dataset
		t.Run(test.name, func(t *testing.T) {
			request := detailRequest(http.MethodPatch, "/api/v1/vms/default/101/cdrom", test.body, cookie)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}

			if recorder.Body.String() == "" || !containsJSONState(recorder.Body.Bytes(), test.wantState) {
				t.Fatalf("body = %s, want state %q", recorder.Body.String(), test.wantState)
			}
		})
	}
}

func containsJSONState(body []byte, state string) bool {
	return string(body) == `{"state":"`+state+`"}` || string(body) == `{"state":"`+state+`"}`+"\n"
}
