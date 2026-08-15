package httpapi

import "net/http"

// adminGuard is the signature of Auth.RequireAdmin: wrap a handler with the
// admin-only identity check (401 if unauthenticated, 403 if non-admin).
type adminGuard func(http.Handler) http.Handler

// registerAdminRoutes wires every admin-only route group behind the
// RequireAdmin guard. Each group is delegated to its own helper so NewRouter
// stays a flat sequence of registrations (SonarQube go:S3776 on NewRouter).
func registerAdminRoutes(mux *http.ServeMux, cfg RouterConfig) {
	guard := cfg.Auth.RequireAdmin
	if cfg.AdminCatalog != nil {
		registerAdminCatalogRoutes(mux, guard, cfg.AdminCatalog)
	}
	if cfg.AdminPolicy != nil {
		registerAdminPolicyRoutes(mux, guard, cfg.AdminPolicy)
	}
	if cfg.AdminPools != nil {
		registerAdminPoolRoutes(mux, guard, cfg.AdminPools)
	}
	if cfg.AdminOps != nil {
		registerAdminOpsRoutes(mux, guard, cfg.AdminOps)
	}
	if cfg.AdminClusters != nil {
		registerAdminClusterRoutes(mux, guard, cfg.AdminClusters)
	}
	if cfg.AdminDocs != nil {
		registerAdminDocsRoutes(mux, guard, cfg.AdminDocs)
	}
}

// registerAdminCatalogRoutes wires the T11 admin catalog endpoints (nodes,
// storages, bridges, isos, profiles, tags) and the T18 admin cloud-init
// template CRUD. Every route is admin-only (FR-008, FR-010).
func registerAdminCatalogRoutes(mux *http.ServeMux, guard adminGuard, h *AdminCatalog) {
	mux.Handle("GET /api/v1/admin/nodes", guard(http.HandlerFunc(h.ServeNodes)))
	mux.Handle("POST /api/v1/admin/nodes/toggle", guard(http.HandlerFunc(h.ServeNodeToggle)))
	mux.Handle("GET /api/v1/admin/storages", guard(http.HandlerFunc(h.ServeStorages)))
	mux.Handle("POST /api/v1/admin/storages/toggle", guard(http.HandlerFunc(h.ServeStorageToggle)))
	mux.Handle("GET /api/v1/admin/bridges", guard(http.HandlerFunc(h.ServeBridges)))
	mux.Handle("POST /api/v1/admin/bridges/toggle", guard(http.HandlerFunc(h.ServeBridgeToggle)))
	mux.Handle("GET /api/v1/admin/isos", guard(http.HandlerFunc(h.ServeISOs)))
	mux.Handle("POST /api/v1/admin/isos/toggle", guard(http.HandlerFunc(h.ServeISOToggle)))
	mux.Handle("GET /api/v1/admin/profiles", guard(http.HandlerFunc(h.ServeProfiles)))
	mux.Handle("POST /api/v1/admin/profiles", guard(http.HandlerFunc(h.ServeProfileCreate)))
	mux.Handle("PUT /api/v1/admin/profiles/{id}", guard(http.HandlerFunc(h.ServeProfileUpdate)))
	mux.Handle("DELETE /api/v1/admin/profiles/{id}", guard(http.HandlerFunc(h.ServeProfileDelete)))
	mux.Handle("POST /api/v1/admin/profiles/{id}/toggle", guard(http.HandlerFunc(h.ServeProfileToggle)))
	mux.Handle("GET /api/v1/admin/tags", guard(http.HandlerFunc(h.ServeTags)))
	mux.Handle("POST /api/v1/admin/tags", guard(http.HandlerFunc(h.ServeTagCreate)))
	mux.Handle("PUT /api/v1/admin/tags/{name}/color", guard(http.HandlerFunc(h.ServeTagColor)))
	mux.Handle("DELETE /api/v1/admin/tags/{name}", guard(http.HandlerFunc(h.ServeTagDelete)))
	mux.Handle("GET /api/v1/admin/cloudinit-templates", guard(http.HandlerFunc(h.ServeCloudInitTemplates)))
	mux.Handle("POST /api/v1/admin/cloudinit-templates", guard(http.HandlerFunc(h.ServeCloudInitTemplateCreate)))
	mux.Handle("PUT /api/v1/admin/cloudinit-templates/{id}", guard(http.HandlerFunc(h.ServeCloudInitTemplateUpdate)))
	mux.Handle("DELETE /api/v1/admin/cloudinit-templates/{id}", guard(http.HandlerFunc(h.ServeCloudInitTemplateDelete)))
	mux.Handle("POST /api/v1/admin/cloudinit-templates/{id}/toggle", guard(http.HandlerFunc(h.ServeCloudInitTemplateToggle)))
}

// registerAdminPolicyRoutes wires the T12 admin policy endpoints (gabarits,
// quotas, node capacity). Admin-only (FR-008).
func registerAdminPolicyRoutes(mux *http.ServeMux, guard adminGuard, h *AdminPolicy) {
	mux.Handle("GET /api/v1/admin/policy", guard(http.HandlerFunc(h.ServePolicy)))
	mux.Handle("PUT /api/v1/admin/policy", guard(http.HandlerFunc(h.ServePolicyUpdate)))
	mux.Handle("GET /api/v1/admin/policy/nodes", guard(http.HandlerFunc(h.ServePolicyNodes)))
	mux.Handle("PUT /api/v1/admin/policy/nodes/{node}", guard(http.HandlerFunc(h.ServePolicyNodeUpdate)))
}

// registerAdminPoolRoutes wires the T13 admin pool endpoints (create, list,
// cascade delete). Admin-only (FR-008).
func registerAdminPoolRoutes(mux *http.ServeMux, guard adminGuard, h *AdminPools) {
	mux.Handle("GET /api/v1/admin/pools", guard(http.HandlerFunc(h.ServeList)))
	mux.Handle("POST /api/v1/admin/pools", guard(http.HandlerFunc(h.ServeCreate)))
	mux.Handle("DELETE /api/v1/admin/pools/{name}", guard(http.HandlerFunc(h.ServeDelete)))
}

// registerAdminOpsRoutes wires the T14 admin exploitation endpoints (audit
// log, dashboard, db export/import, app info). The public version endpoint is
// registered outside the admin guard group (FR-015). Admin-only (FR-008).
func registerAdminOpsRoutes(mux *http.ServeMux, guard adminGuard, h *AdminOps) {
	mux.Handle("GET /api/v1/admin/audit", guard(http.HandlerFunc(h.ServeAudit)))
	mux.Handle("GET /api/v1/admin/dashboard", guard(http.HandlerFunc(h.ServeDashboard)))
	mux.Handle("GET /api/v1/admin/db/export", guard(http.HandlerFunc(h.ServeDBExport)))
	mux.Handle("POST /api/v1/admin/db/import", guard(http.HandlerFunc(h.ServeDBImport)))
	mux.Handle("POST /api/v1/admin/db/import/confirm", guard(http.HandlerFunc(h.ServeDBImportConfirm)))
	mux.Handle("GET /api/v1/admin/appinfo", guard(http.HandlerFunc(h.ServeAppInfo)))
	mux.HandleFunc("GET /api/v1/public/version", h.ServePublicVersion)
}

// registerAdminClusterRoutes wires the admin cluster management endpoints
// (list, create, update, test, oidc, delete). Admin-only (FR-008).
func registerAdminClusterRoutes(mux *http.ServeMux, guard adminGuard, h *AdminClusters) {
	mux.Handle("GET /api/v1/admin/clusters", guard(http.HandlerFunc(h.ServeList)))
	mux.Handle("POST /api/v1/admin/clusters", guard(http.HandlerFunc(h.ServeCreate)))
	mux.Handle("PUT /api/v1/admin/clusters/{name}", guard(http.HandlerFunc(h.ServeUpdate)))
	mux.Handle("POST /api/v1/admin/clusters/{name}/test", guard(http.HandlerFunc(h.ServeTest)))
	mux.Handle("POST /api/v1/admin/clusters/{name}/oidc", guard(http.HandlerFunc(h.ServeOIDC)))
	mux.Handle("DELETE /api/v1/admin/clusters/{name}", guard(http.HandlerFunc(h.ServeDelete)))
}

// registerAdminDocsRoutes wires the issue #53 admin documentation CRUD
// endpoints (list, create, update, delete, toggle). Admin-only (FR-008).
func registerAdminDocsRoutes(mux *http.ServeMux, guard adminGuard, h *AdminDocs) {
	mux.Handle("GET /api/v1/admin/docs", guard(http.HandlerFunc(h.ServeDocsList)))
	mux.Handle("POST /api/v1/admin/docs", guard(http.HandlerFunc(h.ServeDocCreate)))
	mux.Handle("PUT /api/v1/admin/docs/{id}/{lang}", guard(http.HandlerFunc(h.ServeDocUpdate)))
	mux.Handle("DELETE /api/v1/admin/docs/{id}/{lang}", guard(http.HandlerFunc(h.ServeDocDelete)))
	mux.Handle("POST /api/v1/admin/docs/{id}/{lang}/toggle", guard(http.HandlerFunc(h.ServeDocToggle)))
}
