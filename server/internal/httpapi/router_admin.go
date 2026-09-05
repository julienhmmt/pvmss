package httpapi

import "net/http"

// adminRouteProtect wraps an admin handler with the appropriate guards. For
// state-changing methods it also applies per-user rate limiting and CSRF
// validation; GET requests only go through the admin role check.
type adminRouteProtect func(method string, next http.Handler) http.Handler

// registerAdminRoutes wires every admin-only route group behind the
// RequireAdmin guard. Each group is delegated to its own helper so NewRouter
// stays a flat sequence of registrations (SonarQube go:S3776 on NewRouter).
func registerAdminRoutes(mux *http.ServeMux, cfg RouterConfig, adminProtect adminRouteProtect) {
	if cfg.AdminCatalog != nil {
		registerAdminCatalogRoutes(mux, adminProtect, cfg.AdminCatalog)
	}

	if cfg.AdminPolicy != nil {
		registerAdminPolicyRoutes(mux, adminProtect, cfg.AdminPolicy)
	}

	if cfg.AdminPools != nil {
		registerAdminPoolRoutes(mux, adminProtect, cfg.AdminPools)
	}

	if cfg.AdminOps != nil {
		registerAdminOpsRoutes(mux, adminProtect, cfg.AdminOps)
	}

	if cfg.AdminClusters != nil {
		registerAdminClusterRoutes(mux, adminProtect, cfg.AdminClusters)
	}

	if cfg.AdminDocs != nil {
		registerAdminDocsRoutes(mux, adminProtect, cfg.AdminDocs)
	}
}

// registerAdminCatalogRoutes wires the T11 admin catalog endpoints (nodes,
// storages, bridges, isos, profiles, tags) and the T18 admin cloud-init
// template CRUD. Every route is admin-only (FR-008, FR-010).
func registerAdminCatalogRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *AdminCatalog) {
	mux.Handle("GET /api/v1/admin/nodes", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeNodes)))
	mux.Handle("POST /api/v1/admin/nodes/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeNodeToggle)))
	mux.Handle("DELETE /api/v1/admin/nodes/{cluster}/{name}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeNodeDelete)))
	mux.Handle("GET /api/v1/admin/storages", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeStorages)))
	mux.Handle("POST /api/v1/admin/storages/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeStorageToggle)))
	mux.Handle("DELETE /api/v1/admin/storages/{cluster}/{node}/{name}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeStorageDelete)))
	mux.Handle("GET /api/v1/admin/bridges", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeBridges)))
	mux.Handle("POST /api/v1/admin/bridges/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeBridgeToggle)))
	mux.Handle("DELETE /api/v1/admin/bridges/{cluster}/{node}/{name}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeBridgeDelete)))
	mux.Handle("GET /api/v1/admin/isos", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeISOs)))
	mux.Handle("POST /api/v1/admin/isos/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeISOToggle)))
	mux.Handle("DELETE /api/v1/admin/isos/{cluster}/{node}/{storage}/{file}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeISODelete)))
	mux.Handle("GET /api/v1/admin/images", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeImages)))
	mux.Handle("POST /api/v1/admin/images/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeImageToggle)))
	mux.Handle("DELETE /api/v1/admin/images/{cluster}/{node}/{storage}/{file}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeImageDelete)))
	mux.Handle("GET /api/v1/admin/templates", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeTemplates)))
	mux.Handle("POST /api/v1/admin/templates/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeTemplateToggle)))
	mux.Handle("PUT /api/v1/admin/templates/{cluster}/{vmid}", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeTemplateUpdate)))
	mux.Handle("DELETE /api/v1/admin/templates/{cluster}/{vmid}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeTemplateDelete)))
	mux.Handle("GET /api/v1/admin/profiles", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeProfiles)))
	mux.Handle("POST /api/v1/admin/profiles", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeProfileCreate)))
	mux.Handle("PUT /api/v1/admin/profiles/{id}", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeProfileUpdate)))
	mux.Handle("DELETE /api/v1/admin/profiles/{id}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeProfileDelete)))
	mux.Handle("POST /api/v1/admin/profiles/{id}/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeProfileToggle)))
	mux.Handle("GET /api/v1/admin/tags", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeTags)))
	mux.Handle("POST /api/v1/admin/tags", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeTagCreate)))
	mux.Handle("PUT /api/v1/admin/tags/{name}/color", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeTagColor)))
	mux.Handle("DELETE /api/v1/admin/tags/{name}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeTagDelete)))
	mux.Handle("GET /api/v1/admin/cloudinit-templates", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeCloudInitTemplates)))
	mux.Handle("POST /api/v1/admin/cloudinit-templates", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeCloudInitTemplateCreate)))
	mux.Handle("PUT /api/v1/admin/cloudinit-templates/{id}", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeCloudInitTemplateUpdate)))
	mux.Handle("DELETE /api/v1/admin/cloudinit-templates/{id}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeCloudInitTemplateDelete)))
	mux.Handle("POST /api/v1/admin/cloudinit-templates/{id}/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeCloudInitTemplateToggle)))
}

// registerAdminPolicyRoutes wires the T12 admin policy endpoints (gabarits,
// quotas, node capacity). Admin-only (FR-008).
func registerAdminPolicyRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *AdminPolicy) {
	mux.Handle("GET /api/v1/admin/policy", adminProtect(http.MethodGet, http.HandlerFunc(h.ServePolicy)))
	mux.Handle("PUT /api/v1/admin/policy", adminProtect(http.MethodPut, http.HandlerFunc(h.ServePolicyUpdate)))
	mux.Handle("GET /api/v1/admin/policy/nodes", adminProtect(http.MethodGet, http.HandlerFunc(h.ServePolicyNodes)))
	mux.Handle("PUT /api/v1/admin/policy/nodes/{node}", adminProtect(http.MethodPut, http.HandlerFunc(h.ServePolicyNodeUpdate)))
}

// registerAdminPoolRoutes wires the T13 admin pool endpoints (create, list,
// cascade delete). Admin-only (FR-008).
func registerAdminPoolRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *AdminPools) {
	mux.Handle("GET /api/v1/admin/pools", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeList)))
	mux.Handle("POST /api/v1/admin/pools", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeCreate)))
	mux.Handle("DELETE /api/v1/admin/pools/{name}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeDelete)))
}

// registerAdminOpsRoutes wires the T14 admin exploitation endpoints (audit
// log, dashboard, db export/import, app info). The public version endpoint is
// registered outside the admin guard group (FR-015). Admin-only (FR-008).
func registerAdminOpsRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *AdminOps) {
	mux.Handle("GET /api/v1/admin/audit", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeAudit)))
	mux.Handle("GET /api/v1/admin/audit/config", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeAuditConfig)))
	mux.Handle("PUT /api/v1/admin/audit/config", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeAuditConfigUpdate)))
	mux.Handle("GET /api/v1/admin/audit/prune-preview", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeAuditPrunePreview)))
	mux.Handle("GET /api/v1/admin/dashboard", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeDashboard)))
	mux.Handle("GET /api/v1/admin/db/export", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeDBExport)))
	mux.Handle("POST /api/v1/admin/db/import", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeDBImport)))
	mux.Handle("POST /api/v1/admin/db/import/confirm", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeDBImportConfirm)))
	mux.Handle("GET /api/v1/admin/appinfo", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeAppInfo)))
	mux.HandleFunc("GET /api/v1/public/version", h.ServePublicVersion)
}

// registerAdminClusterRoutes wires the admin cluster management endpoints
// (list, create, update, test, oidc, delete). Admin-only (FR-008).
func registerAdminClusterRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *AdminClusters) {
	mux.Handle("GET /api/v1/admin/clusters", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeList)))
	mux.Handle("POST /api/v1/admin/clusters", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeCreate)))
	mux.Handle("PUT /api/v1/admin/clusters/{name}", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeUpdate)))
	mux.Handle("POST /api/v1/admin/clusters/{name}/test", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeTest)))
	mux.Handle("POST /api/v1/admin/clusters/{name}/oidc", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeOIDC)))
	mux.Handle("DELETE /api/v1/admin/clusters/{name}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeDelete)))
}

// registerAdminDocsRoutes wires the issue #53 admin documentation CRUD
// (list, create, update, delete, toggle). Admin-only (FR-008).
func registerAdminDocsRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *AdminDocs) {
	mux.Handle("GET /api/v1/admin/docs", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeDocsList)))
	mux.Handle("POST /api/v1/admin/docs", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeDocCreate)))
	mux.Handle("PUT /api/v1/admin/docs/{id}/{lang}", adminProtect(http.MethodPut, http.HandlerFunc(h.ServeDocUpdate)))
	mux.Handle("DELETE /api/v1/admin/docs/{id}/{lang}", adminProtect(http.MethodDelete, http.HandlerFunc(h.ServeDocDelete)))
	mux.Handle("POST /api/v1/admin/docs/{id}/{lang}/toggle", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeDocToggle)))
}
