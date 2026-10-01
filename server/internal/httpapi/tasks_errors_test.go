package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"pvmss/server/internal/cluster"
	"testing"
)

// failingTaskClient is a full cluster client whose task status read fails
// with a chosen error.
type failingTaskClient struct {
	cluster.Fake
	err error
}

func (c failingTaskClient) TaskStatus(context.Context, string) (cluster.TaskStatus, error) {
	return cluster.TaskStatus{}, c.err
}

//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_StatusErrorsMapToHTTP(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"unknown task", cluster.ErrNotFound, http.StatusNotFound, apiCodeNotFound},
		{"transport failure", errors.New("connection reset"), http.StatusBadGateway, "cluster_error"},
		{"proxmox rejection", &cluster.RejectionError{Status: http.StatusBadRequest, Message: "task log unavailable"}, http.StatusBadGateway, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, authHandler, _ := newTasksHandlerWithRegistry(t,
				map[string]cluster.Client{auditTestCluster: failingTaskClient{err: tc.err}}, nil)

			rec := getTaskWithCluster(t, handler, "UPID:fake", auditTestCluster, aliceCookie(t, authHandler))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_RejectsAnonymousUnknownClusterAndMissingUPID(t *testing.T) {
	handler, authHandler, _ := newTasksHandlerWithRegistry(t,
		map[string]cluster.Client{auditTestCluster: cluster.Fake{}}, nil)
	cookie := aliceCookie(t, authHandler)

	if rec := getTaskWithCluster(t, handler, "UPID:fake", auditTestCluster, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", rec.Code)
	}

	if rec := getTaskWithCluster(t, handler, "UPID:fake", "nonexistent", cookie); rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown cluster status = %d, want 404 or 400", rec.Code)
	}

	if rec := getTaskWithCluster(t, handler, "", auditTestCluster, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing upid status = %d, want 400", rec.Code)
	}
}
