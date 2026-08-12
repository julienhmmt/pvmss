package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	authRateLimitMaxRequests = 10
	authRateLimitWindow      = time.Minute
)

// errorResponse is the public error shape for unknown API paths.
type errorResponse struct {
	Detail string `json:"detail"`
}

// spaHandler serves static assets and falls back to index.html for client-side routes.
type spaHandler struct {
	root  http.FileSystem
	index string
	log   *slog.Logger
}

// RouterConfig configures NewRouter. VMCloudInit, VMCreate, Tasks,
// SnapshotHandlers, VMConsole, AdminCatalog, AdminPolicy, AdminPools, and
// AdminOps are optional — left nil/empty, their routes are simply not
// registered (router tests rely on this to omit handlers without panicking).
type RouterConfig struct {
	Health           http.Handler
	ClusterNodes     http.Handler
	ClusterRefresh   http.Handler
	VMs              http.Handler
	VMDetail         http.Handler
	VMCloudInit      *VMCloudInit
	VMCreate         *VMCreate
	Tasks            *Tasks
	Auth             *Auth
	WebBuildDir      string
	Log              *slog.Logger
	SnapshotHandlers []*VMSnapshots
	VMConsole        *VMConsole
	AdminCatalog     *AdminCatalog
	AdminPolicy      *AdminPolicy
	AdminPools       *AdminPools
	AdminOps         *AdminOps
	AdminClusters    *AdminClusters
}

// NewRouter wires the public API and the static SPA handler from cfg.
//
//nolint:gocyclo,funlen // route registration is inherently a long switch on cfg fields
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /health", cfg.Health)
	mux.Handle("GET /api/v1/cluster/nodes", cfg.Auth.Require(cfg.ClusterNodes))
	mux.Handle("POST /api/v1/cluster/refresh", cfg.Auth.Require(cfg.ClusterRefresh))
	// Not wrapped in auth.Require: the handler needs the resolved Identity
	// itself (for scope enforcement) and calls h.auth.Principal(r) directly,
	// returning 401 on its own — wrapping would just re-run the same check.
	mux.Handle("GET /api/v1/vms", cfg.VMs)
	// VM creation + catalog + task polling — same Principal pattern as above.
	if cfg.VMCreate != nil {
		mux.Handle("POST /api/v1/vms", cfg.VMCreate)
		mux.Handle("GET /api/v1/vm-create/catalog", http.HandlerFunc(cfg.VMCreate.ServeCatalog))
	}

	if cfg.Tasks != nil {
		mux.Handle("GET /api/v1/tasks/{upid}", cfg.Tasks)
	}
	// VM detail + actions + delete + patch — all gated by vm.Resolve() inside
	// the handler. Same reason as GET /vms: not wrapped in auth.Require.
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}", cfg.VMDetail)
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/actions", cfg.VMDetail)
	mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}", cfg.VMDetail)
	mux.Handle("PATCH /api/v1/vms/{cluster}/{vmid}", cfg.VMDetail)
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/hardware-options", cfg.VMDetail)
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/disks", cfg.VMDetail)
	mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/disks/{diskKey}/resize", cfg.VMDetail)
	mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}/disks/{diskKey}", cfg.VMDetail)
	mux.Handle("PATCH /api/v1/vms/{cluster}/{vmid}/cdrom", cfg.VMDetail)
	mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/network", cfg.VMDetail)
	mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/hardware", cfg.VMDetail)

	if cfg.VMCloudInit != nil {
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/cloudinit", cfg.VMCloudInit)
		mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/cloudinit", cfg.VMCloudInit)
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet", cfg.VMCloudInit)
		mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet", cfg.VMCloudInit)
	}

	if len(cfg.SnapshotHandlers) > 0 && cfg.SnapshotHandlers[0] != nil {
		snapshots := cfg.SnapshotHandlers[0]
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/snapshots", snapshots)
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/snapshots", snapshots)
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/snapshots/{name}/rollback", snapshots)
		mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}/snapshots/{name}", snapshots)
	}

	if cfg.VMConsole != nil {
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/vnc-ticket", cfg.VMConsole)
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/console/websocket", cfg.VMConsole)
	}

	// Unauthenticated credential-check endpoints get a per-IP rate limit —
	// nothing else gates repeated guesses against them. The pre-login cluster
	// list and OIDC trigger are also unauthenticated and disclose cluster
	// names, so they share the same limiter to bound enumeration/abuse.
	authLimiter := newIPRateLimiter(authRateLimitMaxRequests, authRateLimitWindow)
	mux.Handle("POST /api/v1/auth/login", authLimiter.middleware(http.HandlerFunc(cfg.Auth.Login)))
	mux.Handle("POST /api/v1/auth/admin-login", authLimiter.middleware(http.HandlerFunc(cfg.Auth.AdminLogin)))
	mux.HandleFunc("GET /api/v1/auth/me", cfg.Auth.Me)
	mux.Handle("GET /api/v1/auth/clusters", authLimiter.middleware(http.HandlerFunc(cfg.Auth.ServeClusters)))
	mux.Handle("POST /api/v1/auth/oidc", authLimiter.middleware(http.HandlerFunc(cfg.Auth.OIDC)))
	mux.HandleFunc("POST /api/v1/auth/logout", cfg.Auth.Logout)
	mux.HandleFunc("POST /api/v1/auth/tokens", cfg.Auth.CreateToken)
	mux.HandleFunc("GET /api/v1/auth/tokens", cfg.Auth.ListTokens)
	mux.HandleFunc("DELETE /api/v1/auth/tokens/{id}", cfg.Auth.RevokeToken)
	mux.HandleFunc("POST /api/v1/auth/password", cfg.Auth.ChangePassword)

	// T11 admin catalog — every route is admin-only (FR-008).
	if cfg.AdminCatalog != nil {
		adminGuard := cfg.Auth.RequireAdmin
		mux.Handle("GET /api/v1/admin/nodes", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeNodes)))
		mux.Handle("POST /api/v1/admin/nodes/toggle", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeNodeToggle)))
		mux.Handle("GET /api/v1/admin/storages", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeStorages)))
		mux.Handle("POST /api/v1/admin/storages/toggle", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeStorageToggle)))
		mux.Handle("GET /api/v1/admin/bridges", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeBridges)))
		mux.Handle("POST /api/v1/admin/bridges/toggle", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeBridgeToggle)))
		mux.Handle("GET /api/v1/admin/isos", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeISOs)))
		mux.Handle("POST /api/v1/admin/isos/toggle", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeISOToggle)))
		mux.Handle("GET /api/v1/admin/profiles", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeProfiles)))
		mux.Handle("POST /api/v1/admin/profiles", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeProfileCreate)))
		mux.Handle("PUT /api/v1/admin/profiles/{id}", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeProfileUpdate)))
		mux.Handle("DELETE /api/v1/admin/profiles/{id}", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeProfileDelete)))
		mux.Handle("POST /api/v1/admin/profiles/{id}/toggle", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeProfileToggle)))
		mux.Handle("GET /api/v1/admin/tags", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeTags)))
		mux.Handle("POST /api/v1/admin/tags", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeTagCreate)))
		mux.Handle("PUT /api/v1/admin/tags/{name}/color", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeTagColor)))
		mux.Handle("DELETE /api/v1/admin/tags/{name}", adminGuard(http.HandlerFunc(cfg.AdminCatalog.ServeTagDelete)))
	}

	// T12 admin policies — gabarits, quotas, node capacity; admin-only.
	if cfg.AdminPolicy != nil {
		adminGuard := cfg.Auth.RequireAdmin
		mux.Handle("GET /api/v1/admin/policy", adminGuard(http.HandlerFunc(cfg.AdminPolicy.ServePolicy)))
		mux.Handle("PUT /api/v1/admin/policy", adminGuard(http.HandlerFunc(cfg.AdminPolicy.ServePolicyUpdate)))
		mux.Handle("GET /api/v1/admin/policy/nodes", adminGuard(http.HandlerFunc(cfg.AdminPolicy.ServePolicyNodes)))
		mux.Handle("PUT /api/v1/admin/policy/nodes/{node}", adminGuard(http.HandlerFunc(cfg.AdminPolicy.ServePolicyNodeUpdate)))
	}

	// T13 admin pools — create, list, cascade delete; admin-only.
	if cfg.AdminPools != nil {
		adminGuard := cfg.Auth.RequireAdmin
		mux.Handle("GET /api/v1/admin/pools", adminGuard(http.HandlerFunc(cfg.AdminPools.ServeList)))
		mux.Handle("POST /api/v1/admin/pools", adminGuard(http.HandlerFunc(cfg.AdminPools.ServeCreate)))
		mux.Handle("DELETE /api/v1/admin/pools/{name}", adminGuard(http.HandlerFunc(cfg.AdminPools.ServeDelete)))
	}

	// T14 admin exploitation — audit log, dashboard, db export/import, app
	// info; admin-only. The public version endpoint is registered outside
	// the admin guard group (FR-015).
	if cfg.AdminOps != nil {
		adminGuard := cfg.Auth.RequireAdmin
		mux.Handle("GET /api/v1/admin/audit", adminGuard(http.HandlerFunc(cfg.AdminOps.ServeAudit)))
		mux.Handle("GET /api/v1/admin/dashboard", adminGuard(http.HandlerFunc(cfg.AdminOps.ServeDashboard)))
		mux.Handle("GET /api/v1/admin/db/export", adminGuard(http.HandlerFunc(cfg.AdminOps.ServeDBExport)))
		mux.Handle("POST /api/v1/admin/db/import", adminGuard(http.HandlerFunc(cfg.AdminOps.ServeDBImport)))
		mux.Handle("POST /api/v1/admin/db/import/confirm", adminGuard(http.HandlerFunc(cfg.AdminOps.ServeDBImportConfirm)))
		mux.Handle("GET /api/v1/admin/appinfo", adminGuard(http.HandlerFunc(cfg.AdminOps.ServeAppInfo)))
		mux.HandleFunc("GET /api/v1/public/version", cfg.AdminOps.ServePublicVersion)
	}

	if cfg.AdminClusters != nil {
		adminGuard := cfg.Auth.RequireAdmin
		mux.Handle("GET /api/v1/admin/clusters", adminGuard(http.HandlerFunc(cfg.AdminClusters.ServeList)))
		mux.Handle("POST /api/v1/admin/clusters", adminGuard(http.HandlerFunc(cfg.AdminClusters.ServeCreate)))
		mux.Handle("PUT /api/v1/admin/clusters/{name}", adminGuard(http.HandlerFunc(cfg.AdminClusters.ServeUpdate)))
		mux.Handle("POST /api/v1/admin/clusters/{name}/test", adminGuard(http.HandlerFunc(cfg.AdminClusters.ServeTest)))
		mux.Handle("POST /api/v1/admin/clusters/{name}/oidc", adminGuard(http.HandlerFunc(cfg.AdminClusters.ServeOIDC)))
		mux.Handle("DELETE /api/v1/admin/clusters/{name}", adminGuard(http.HandlerFunc(cfg.AdminClusters.ServeDelete)))
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mux.Handle(method+" /api/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if err := writeError(w, http.StatusNotFound, "unknown API path"); err != nil {
				cfg.Log.Error("failed to write API 404", "component", "httpapi", "error", err)
			}
		}))
	}

	if cfg.WebBuildDir != "" {
		spa := &spaHandler{
			root:  http.Dir(cfg.WebBuildDir),
			index: "/index.html",
			log:   cfg.Log,
		}
		mux.Handle("GET /", spa)
	}

	// Wrap the entire mux with security headers so every response (API and
	// SPA) gets CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy,
	// Permissions-Policy, HSTS, and cache-control for API paths.
	return withSecurityHeaders(mux)
}

func (s *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(r.URL.Path)
	if p == "." {
		p = "/"
	}

	// Asset paths and files with an extension must exist; never fall back.
	if strings.HasPrefix(p, "/_app/") || path.Ext(p) != "" {
		if err := s.serveFile(w, r, p); err != nil {
			if writeErr := writeTextError(w, http.StatusNotFound, "asset not found"); writeErr != nil {
				s.log.Error("failed to write asset 404", "component", "httpapi", "path", p, "error", writeErr)
			}
		}

		return
	}

	// For client-side routes, try the file first, then fall back to index.html.
	if err := s.serveFile(w, r, p); err == nil {
		return
	}

	if err := s.serveFile(w, r, s.index); err != nil {
		if writeErr := writeTextError(w, http.StatusNotFound, "shell not found"); writeErr != nil {
			s.log.Error("failed to write shell 404", "component", "httpapi", "path", p, "error", writeErr)
		}
	}
}

func (s *spaHandler) serveFile(w http.ResponseWriter, r *http.Request, name string) error {
	f, err := s.root.Open(name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.log.Error("failed to open static file", "component", "httpapi", "path", name, "error", err)
		}

		return err
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return err
	}

	if st.IsDir() {
		return fs.ErrNotExist
	}

	http.ServeContent(
		w,
		r,
		name,
		st.ModTime(),
		f,
	)

	return nil
}

// writeJSON writes a JSON response with the given status code and returns any write error.
func writeJSON(w http.ResponseWriter, status int, body []byte) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(body)

	return err
}

// writeError writes a JSON error response with the given status code and detail.
func writeError(w http.ResponseWriter, status int, detail string) error {
	body, err := json.Marshal(errorResponse{Detail: detail})
	if err != nil {
		return fmt.Errorf("marshal error response: %w", err)
	}

	return writeJSON(w, status, body)
}

// writeTextError writes a plain-text error response with the given status code and detail.
func writeTextError(w http.ResponseWriter, status int, detail string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(detail))

	return err
}
