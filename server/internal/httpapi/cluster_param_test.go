//nolint:wsl_v5 // table-driven parameter cases keep expected outcomes adjacent
package httpapi_test

import (
	"context"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"testing"
)

type clusterNameLister struct {
	names []string
}

func (lister clusterNameLister) List() []string {
	return lister.names
}

//nolint:paralleltest,goconst // table cases share the lister fixture; wire values are explicit
func TestResolveClusterParam(t *testing.T) {
	tests := []struct {
		name      string
		clusters  []string
		query     string
		want      string
		wantError bool
	}{
		{name: "sole cluster defaults", clusters: []string{"default"}, want: "default"},
		{name: "multiple clusters require choice", clusters: []string{"default", "secondary"}, wantError: true},
		{name: "explicit choice passes through", clusters: []string{"default", "secondary"}, query: "?cluster=secondary", want: "secondary"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/admin/nodes"+test.query, nil)
			got, err := httpapi.ResolveClusterParam(request, clusterNameLister{names: test.clusters})
			if test.wantError {
				if err == nil {
					t.Fatal("ResolveClusterParam returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveClusterParam: %v", err)
			}
			if got != test.want {
				t.Fatalf("cluster = %q, want %q", got, test.want)
			}
		})
	}
}
