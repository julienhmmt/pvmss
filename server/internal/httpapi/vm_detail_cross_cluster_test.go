//nolint:wsl_v5 // cross-cluster HTTP assertions stay in one scenario
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
)

const crossSecondaryCluster = "secondary"

//nolint:paralleltest // detail fixture shares fake authentication state
func TestVMDetail_CrossClusterVMIDReturnsDistinctEntities(t *testing.T) {
	defaultIndex := inventory.BuildIndexForCluster(auditTestCluster, cluster.Snapshot{VMs: []cluster.VM{{VMID: 101, Name: "default-web", Node: "default-node", Pool: cluster.FakePoolAlice, Tags: []string{"pvmss"}}}})
	secondaryIndex := inventory.BuildIndexForCluster(crossSecondaryCluster, cluster.Snapshot{VMs: []cluster.VM{{VMID: 101, Name: "secondary-web", Node: "secondary-node", Pool: cluster.FakePoolAlice, Tags: []string{"pvmss"}}}})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{auditTestCluster: &defaultIndex, crossSecondaryCluster: &secondaryIndex})
	projection := inventory.NewProjectionFromIndex(&defaultIndex)
	authHandler := newAuthHandler(t)
	handler := httpapi.NewVMDetailWithRegistry(registry, projection, authHandler, cluster.Fake{}, nil, nil, slog.Default())
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	get := func(name string) string {
		t.Helper()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/vms/"+name+"/101", nil)
		request.SetPathValue("cluster", name)
		request.SetPathValue("vmid", "101")
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d: %s", name, response.Code, response.Body.String())
		}
		var body struct {
			Cluster string `json:"cluster"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if body.Cluster != name {
			t.Fatalf("cluster = %q, want %q", body.Cluster, name)
		}
		return body.Name
	}

	if defaultName, secondaryName := get(auditTestCluster), get(crossSecondaryCluster); defaultName == secondaryName {
		t.Fatalf("same VMID resolved to same name: %q", defaultName)
	}
}
