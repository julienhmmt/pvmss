package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type clusterTestEntry struct {
	TestedAt   string `json:"testedAt"`
	Status     string `json:"status"`
	Actor      string `json:"actor"`
	DurationMs *int64 `json:"durationMs"`
}

//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_TestHistoryListsNewestWithActor(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)
	testPath := "/api/v1/admin/clusters/default/test"
	tested := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeTest, HTTPMethod: http.MethodPost, Path: testPath, Name: auditTestCluster})

	var testResult struct {
		Entry *clusterTestEntry `json:"entry"`
	}
	if err := json.Unmarshal(tested.Body.Bytes(), &testResult); err != nil || testResult.Entry == nil || testResult.Entry.Actor == "" || testResult.Entry.DurationMs == nil {
		t.Fatalf("test response entry = %+v (err %v), want actor and duration: %s", testResult.Entry, err, tested.Body.String())
	}

	history := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeTestHistory, HTTPMethod: http.MethodGet, Path: "/api/v1/admin/clusters/default/tests?limit=10", Name: auditTestCluster})
	if history.Code != http.StatusOK {
		t.Fatalf("history status = %d: %s", history.Code, history.Body.String())
	}

	var payload struct {
		Tests []clusterTestEntry `json:"tests"`
	}
	if err := json.Unmarshal(history.Body.Bytes(), &payload); err != nil || len(payload.Tests) == 0 || len(payload.Tests) > 10 {
		t.Fatalf("history = %+v (err %v), want 1..10 rows", payload.Tests, err)
	}

	if payload.Tests[0].TestedAt != testResult.Entry.TestedAt {
		t.Fatalf("newest row = %+v, want the test just run %+v", payload.Tests[0], *testResult.Entry)
	}
}

//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_TestHistoryRejectsBadLimitAndUnknownCluster(t *testing.T) {
	fixture := newAdminClusterFixture(t)

	cookie := adminClusterCookie(t, fixture.auth)
	for _, tc := range []struct {
		path, name string
		want       int
	}{
		{"/api/v1/admin/clusters/default/tests?limit=7", auditTestCluster, http.StatusBadRequest},
		{"/api/v1/admin/clusters/default/tests?limit=abc", auditTestCluster, http.StatusBadRequest},
		{"/api/v1/admin/clusters/nope/tests", "nope", http.StatusNotFound},
	} {
		response := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeTestHistory, HTTPMethod: http.MethodGet, Path: tc.path, Name: tc.name})
		if response.Code != tc.want {
			t.Errorf("%s = %d, want %d", tc.path, response.Code, tc.want)
		}
	}
}
