package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminPolicyNodes_Guards(t *testing.T) {
	handler, _ := newPolicyHandler(t)

	cases := []struct {
		name  string
		serve http.HandlerFunc
		req   func() *http.Request
		want  int
	}{
		{"list rejects POST", handler.ServePolicyNodes, func() *http.Request { return nodePolicyRequest(http.MethodPost, "", "") }, http.StatusMethodNotAllowed},
		{"update rejects GET", handler.ServePolicyNodeUpdate, func() *http.Request { return nodePolicyRequest(http.MethodGet, "pve-node-01", "") }, http.StatusMethodNotAllowed},
		{"update needs a node", handler.ServePolicyNodeUpdate, func() *http.Request { return nodePolicyRequest(http.MethodPut, "", `{"maxVms":1}`) }, http.StatusBadRequest},
		{"update rejects bad body", handler.ServePolicyNodeUpdate, func() *http.Request { return nodePolicyRequest(http.MethodPut, "pve-node-01", badJSONBody) }, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.serve(rec, tc.req())

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func nodePolicyRequest(method, node, body string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, "/api/v1/admin/policy/nodes", strings.NewReader(body))
	if node != "" {
		req.SetPathValue("node", node)
	}

	return req
}
