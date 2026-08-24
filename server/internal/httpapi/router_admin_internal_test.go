package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterAdminRoutes_AllGroupsRegistered(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	cfg := RouterConfig{
		Auth:          &Auth{},
		AdminCatalog:  &AdminCatalog{},
		AdminPolicy:   &AdminPolicy{},
		AdminPools:    &AdminPools{},
		AdminOps:      &AdminOps{},
		AdminClusters: &AdminClusters{},
		AdminDocs:     &AdminDocs{},
	}

	registerAdminRoutes(mux, cfg, func(_ string, next http.Handler) http.Handler { return next })

	wantRoutes := []string{
		"GET /api/v1/admin/nodes",
		"POST /api/v1/admin/nodes/toggle",
		"GET /api/v1/admin/storages",
		"POST /api/v1/admin/storages/toggle",
		"GET /api/v1/admin/bridges",
		"POST /api/v1/admin/bridges/toggle",
		"GET /api/v1/admin/isos",
		"POST /api/v1/admin/isos/toggle",
		"GET /api/v1/admin/profiles",
		"POST /api/v1/admin/profiles",
		"PUT /api/v1/admin/profiles/{id}",
		"DELETE /api/v1/admin/profiles/{id}",
		"POST /api/v1/admin/profiles/{id}/toggle",
		"GET /api/v1/admin/tags",
		"POST /api/v1/admin/tags",
		"PUT /api/v1/admin/tags/{name}/color",
		"DELETE /api/v1/admin/tags/{name}",
		"GET /api/v1/admin/cloudinit-templates",
		"POST /api/v1/admin/cloudinit-templates",
		"PUT /api/v1/admin/cloudinit-templates/{id}",
		"DELETE /api/v1/admin/cloudinit-templates/{id}",
		"POST /api/v1/admin/cloudinit-templates/{id}/toggle",
		"GET /api/v1/admin/policy",
		"PUT /api/v1/admin/policy",
		"GET /api/v1/admin/policy/nodes",
		"PUT /api/v1/admin/policy/nodes/{node}",
		"GET /api/v1/admin/pools",
		"POST /api/v1/admin/pools",
		"DELETE /api/v1/admin/pools/{name}",
		"GET /api/v1/admin/audit",
		"GET /api/v1/admin/dashboard",
		"GET /api/v1/admin/db/export",
		"POST /api/v1/admin/db/import",
		"POST /api/v1/admin/db/import/confirm",
		"GET /api/v1/admin/appinfo",
		"GET /api/v1/public/version",
		"GET /api/v1/admin/clusters",
		"POST /api/v1/admin/clusters",
		"PUT /api/v1/admin/clusters/{name}",
		"POST /api/v1/admin/clusters/{name}/test",
		"POST /api/v1/admin/clusters/{name}/oidc",
		"DELETE /api/v1/admin/clusters/{name}",
		"GET /api/v1/admin/docs",
		"POST /api/v1/admin/docs",
		"PUT /api/v1/admin/docs/{id}/{lang}",
		"DELETE /api/v1/admin/docs/{id}/{lang}",
		"POST /api/v1/admin/docs/{id}/{lang}/toggle",
	}

	for _, pattern := range wantRoutes {
		req := httptest.NewRequest(methodFromPattern(pattern), pathFromPattern(pattern), nil)

		_, pattern2 := mux.Handler(req)
		if pattern2 != pattern {
			t.Errorf("route %q not registered (got pattern %q)", pattern, pattern2)
		}
	}
}

func TestRegisterAdminRoutes_NilGroupsSkipped(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	cfg := RouterConfig{Auth: &Auth{}}

	registerAdminRoutes(mux, cfg, func(_ string, next http.Handler) http.Handler { return next })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)

	_, pattern := mux.Handler(req)
	if pattern != "" {
		t.Errorf("expected no route for /api/v1/admin/nodes when AdminCatalog is nil, got %q", pattern)
	}
}

func methodFromPattern(pattern string) string {
	idx := 0
	for idx < len(pattern) && pattern[idx] != ' ' {
		idx++
	}

	if idx == len(pattern) {
		return http.MethodGet
	}

	return pattern[:idx]
}

func pathFromPattern(pattern string) string {
	idx := 0
	for idx < len(pattern) && pattern[idx] != ' ' {
		idx++
	}

	if idx == len(pattern) {
		return pattern
	}

	path := pattern[idx+1:]
	path = substitutePathParams(path)

	return path
}

func substitutePathParams(path string) string {
	result := []byte{}
	inParam := false

	for _, c := range path {
		if c == '{' {
			inParam = true

			result = append(result, 'x')

			continue
		}

		if c == '}' {
			inParam = false
			continue
		}

		if !inParam {
			result = append(result, byte(c))
		}
	}

	return string(result)
}
