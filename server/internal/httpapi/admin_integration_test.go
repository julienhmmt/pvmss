package httpapi_test

import (
	"context"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"testing"
)

// TestAdminIntegration_ToggleNodeThenApprovedResourcesIncludesIt — T017/SC-002:
// toggle a node on via the admin handler, then call T06's
// catalog.ApprovedResources directly (no HTTP) and confirm it includes the
// newly approved node — the cross-tranche proof that both surfaces share one
// source of truth.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminIntegration_ToggleNodeThenApprovedResourcesIncludesIt(t *testing.T) {
	handler, authHandler, st := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Toggle pve-node-03 on.
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-03","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	// Cross-tranche: T06's ApprovedResources now includes pve-node-03.
	resources, err := catalog.ApprovedResources(context.Background(), st, "default")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	found := false

	for _, n := range resources.Nodes {
		if n.Name == cluster.FakeNode03 {
			found = true
		}
	}

	if !found {
		t.Fatal("pve-node-03 not in ApprovedResources after admin toggle — cross-tranche proof failed")
	}
}
