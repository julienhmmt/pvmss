package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const scheduleTestPath = "/api/v1/vms/default/101/snapshot-schedule"

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_Schedule_SetListClear(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	serve := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, snapshotRequest(method, path, body, cookie))

		return rec
	}

	set := serve(http.MethodPut, scheduleTestPath, `{"frequency":"daily","retain":3}`)
	if set.Code != http.StatusOK {
		t.Fatalf("PUT status = %d: %s", set.Code, set.Body.String())
	}

	var dto struct {
		Frequency string `json:"frequency"`
		Retain    int    `json:"retain"`
	}
	if err := json.Unmarshal(set.Body.Bytes(), &dto); err != nil || dto.Frequency != "daily" || dto.Retain != 3 {
		t.Fatalf("PUT body = %s (err %v), want daily/3", set.Body.String(), err)
	}

	list := serve(http.MethodGet, "/api/v1/vms/default/101/snapshots", "")

	var listed struct {
		Schedule *struct {
			Frequency string `json:"frequency"`
		} `json:"schedule"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || listed.Schedule == nil || listed.Schedule.Frequency != "daily" {
		t.Errorf("list schedule = %s (err %v), want daily", list.Body.String(), err)
	}

	for _, tc := range []struct{ body, code string }{
		{`{"frequency":"hourly","retain":3}`, "invalid_schedule"},
		{`{"frequency":"daily","retain":99}`, "retain_exceeds_max_snapshots"},
	} {
		rec := serve(http.MethodPut, scheduleTestPath, tc.body)

		var e snapshotErrorResponse

		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if rec.Code != http.StatusBadRequest || e.Code != tc.code {
			t.Errorf("PUT %s = %d %q, want 400 %q", tc.body, rec.Code, e.Code, tc.code)
		}
	}

	if rec := serve(http.MethodPut, scheduleTestPath, `not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json status = %d, want 400", rec.Code)
	}

	if rec := serve(http.MethodDelete, scheduleTestPath, ""); rec.Code != http.StatusNoContent {
		t.Errorf("DELETE status = %d: %s", rec.Code, rec.Body.String())
	}
}
