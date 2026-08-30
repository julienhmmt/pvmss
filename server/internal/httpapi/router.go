package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"pvmss/server/internal/store"
	"strings"
	"time"
)

const (
	authRateLimitMaxRequests    = 10
	authRateLimitWindow         = time.Minute
	vmWriteRateLimitMaxRequests = 30
	vmWriteRateLimitWindow      = time.Minute
	// vmStatusRateLimitMaxRequests allows the frontend convergence loop to
	// poll batch live status every 1.5s for up to 30s per action (20 polls),
	// with headroom for concurrent actions.
	vmStatusRateLimitMaxRequests    = 120
	vmStatusRateLimitWindow         = time.Minute
	clusterTestRateLimitMaxRequests = 10
	clusterTestRateLimitWindow      = time.Minute
	adminWriteRateLimitMaxRequests  = 60
	adminWriteRateLimitWindow       = time.Minute
	authWriteRateLimitMaxRequests   = 30
	authWriteRateLimitWindow        = time.Minute
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
	VMBulk           *VMBulk
	VMStatusBatch    *VMStatusBatch
	VMCloudInit      *VMCloudInit
	VMCreate         *VMCreate
	Tasks            *Tasks
	Auth             *Auth
	WebBuildDir      string
	Log              *slog.Logger
	SnapshotHandlers []*VMSnapshots
	VMConsole        *VMConsole
	VMSerialConsole  *VMSerialConsole
	VMMetrics        *VMMetrics
	AdminCatalog     *AdminCatalog
	AdminPolicy      *AdminPolicy
	AdminPools       *AdminPools
	AdminOps         *AdminOps
	AdminClusters    *AdminClusters
	Docs             *DocsAPIHandler
	AdminDocs        *AdminDocs
	Store            *store.Store
	// TrustedProxyHops is forwarded to the rate limiters so clientIP resolves
	// the real user IP behind a Kubernetes ingress via X-Forwarded-For.
	// Defaults to 0 (use RemoteAddr directly) when not set by the caller.
	TrustedProxyHops int
}

// NewRouter wires the public API and the static SPA handler from cfg.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	hops := cfg.TrustedProxyHops
	csrf := newCSRFMiddleware(cfg.Auth, cfg.Store, hops)
	vmWriteLimiter := newUserRateLimiter(vmWriteRateLimitMaxRequests, vmWriteRateLimitWindow, hops, cfg.Store)
	vmStatusLimiter := newUserRateLimiter(vmStatusRateLimitMaxRequests, vmStatusRateLimitWindow, hops, cfg.Store)
	clusterTestLimiter := newUserRateLimiter(clusterTestRateLimitMaxRequests, clusterTestRateLimitWindow, hops, cfg.Store)
	adminWriteLimiter := newUserRateLimiter(adminWriteRateLimitMaxRequests, adminWriteRateLimitWindow, hops, cfg.Store)
	authWriteLimiter := newUserRateLimiter(authWriteRateLimitMaxRequests, authWriteRateLimitWindow, hops, cfg.Store)

	// protect combines per-user rate limiting (outer) and CSRF validation (inner)
	// for state-changing browser requests.
	protect := func(next http.Handler, limiter *userRateLimiter) http.Handler {
		return limiter.middleware(cfg.Auth, csrf(next))
	}

	adminProtect := func(method string, next http.Handler) http.Handler {
		return newAdminProtect(cfg, method, next, adminWriteLimiter, csrf, hops)
	}

	mux.Handle("GET /health", cfg.Health)
	mux.Handle("GET /api/v1/cluster/nodes", cfg.Auth.Require(cfg.ClusterNodes))
	mux.Handle("POST /api/v1/cluster/refresh", protect(cfg.Auth.Require(cfg.ClusterRefresh), clusterTestLimiter))

	registerVMRoutes(mux, cfg, protect, vmWriteLimiter, vmStatusLimiter)
	registerAuthRoutes(mux, cfg, protect, authWriteLimiter, hops)

	// Issue #53 public documentation — audience-filtered list and rendered
	// single-page view. Not wrapped in auth.Require: the handler resolves the
	// caller itself (to hide admin-audience pages from non-admins) and issues
	// its own 401/403 on admin-audience pages.
	if cfg.Docs != nil {
		mux.Handle("GET /api/v1/docs", http.HandlerFunc(cfg.Docs.ServeDocsList))
		mux.Handle("GET /api/v1/docs/{id}", http.HandlerFunc(cfg.Docs.ServeDoc))
	}

	// Admin-only route groups (T11/T12/T13/T14/T18/issue#53). Each group is
	// wired by its own helper behind the RequireAdmin guard (FR-008). Extracted
	// from NewRouter to keep its Cognitive Complexity under the SonarQube
	// go:S3776 threshold.
	registerAdminRoutes(mux, cfg, adminProtect)

	registerAPINotFound(mux, cfg)
	registerSPA(mux, cfg)

	// Wrap the entire mux with security headers so every response (API and
	// SPA) gets CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy,
	// Permissions-Policy, HSTS, and cache-control for API paths.
	return withSecurityHeaders(mux)
}

// protectFunc is the signature of NewRouter's `protect` closure, factored out
// so the extracted route-registration helpers can take it as a parameter.
type protectFunc func(next http.Handler, limiter *userRateLimiter) http.Handler

// registerVMRoutes wires the VM, task, console, serial, snapshot, and metrics
// routes. Extracted from NewRouter to keep its cyclomatic complexity under
// gocyclo's ceiling. The handlers call h.auth.Principal(r) directly and return
// 401 on their own, so they are not wrapped in auth.Require.
func registerVMRoutes(mux *http.ServeMux, cfg RouterConfig, protect protectFunc, vmWriteLimiter, vmStatusLimiter *userRateLimiter) {
	// Not wrapped in auth.Require: the handler needs the resolved Identity
	// itself (for scope enforcement) and calls h.auth.Principal(r) directly,
	// returning 401 on its own — wrapping would just re-run the same check.
	mux.Handle("GET /api/v1/vms", cfg.VMs)
	// T17 bulk VM power actions — same Principal pattern as VM list/detail:
	// the handler calls h.auth.Principal(r) directly and returns 401 on its
	// own, so it is not wrapped in auth.Require. Registered before the
	// {cluster}/{vmid} pattern so the literal "bulk-action" segment wins.
	if cfg.VMBulk != nil {
		mux.Handle("POST /api/v1/vms/bulk-action", protect(cfg.VMBulk, vmWriteLimiter))
	}
	// Batch live-status read (ADR 0001). Registered before the
	// {cluster}/{vmid} pattern so the literal "status" segment wins, same
	// reason as bulk-action above. Wrapped in protect for CSRF (it takes a
	// POST body) with a read-oriented rate limit.
	if cfg.VMStatusBatch != nil {
		mux.Handle("POST /api/v1/vms/status", protect(cfg.VMStatusBatch, vmStatusLimiter))
	}
	// VM creation + catalog + task polling — same Principal pattern as above.
	if cfg.VMCreate != nil {
		mux.Handle("POST /api/v1/vms", protect(cfg.VMCreate, vmWriteLimiter))
		mux.Handle("GET /api/v1/vm-create/catalog", http.HandlerFunc(cfg.VMCreate.ServeCatalog))
	}

	if cfg.Tasks != nil {
		mux.Handle("GET /api/v1/tasks/{upid}", cfg.Tasks)
	}
	// VM detail + actions + delete + patch — all gated by vm.Resolve() inside
	// the handler. Same reason as GET /vms: not wrapped in auth.Require.
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}", cfg.VMDetail)
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/actions", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("PATCH /api/v1/vms/{cluster}/{vmid}", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/hardware-options", cfg.VMDetail)
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/status", cfg.VMDetail)
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/disks", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/disks/{diskKey}/resize", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}/disks/{diskKey}", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("PATCH /api/v1/vms/{cluster}/{vmid}/cdrom", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/network", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/hardware", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/serial", protect(cfg.VMDetail, vmWriteLimiter))
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/audit", cfg.VMDetail)

	if cfg.VMCloudInit != nil {
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/cloudinit", cfg.VMCloudInit)
		mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/cloudinit", protect(cfg.VMCloudInit, vmWriteLimiter))
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet", cfg.VMCloudInit)
		mux.Handle("PUT /api/v1/vms/{cluster}/{vmid}/cloudinit/snippet", protect(cfg.VMCloudInit, vmWriteLimiter))
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/cloudinit/ssh-keys", protect(cfg.VMCloudInit, vmWriteLimiter))
	}

	if len(cfg.SnapshotHandlers) > 0 && cfg.SnapshotHandlers[0] != nil {
		snapshots := cfg.SnapshotHandlers[0]
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/snapshots", snapshots)
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/snapshots", protect(snapshots, vmWriteLimiter))
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/snapshots/{name}/rollback", protect(snapshots, vmWriteLimiter))
		mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}/snapshots/{name}", protect(snapshots, vmWriteLimiter))
	}

	if cfg.VMConsole != nil {
		mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/vnc-ticket", protect(cfg.VMConsole, vmWriteLimiter))
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/console/websocket", cfg.VMConsole)
	}

	registerSerialConsoleRoutes(mux, cfg.VMSerialConsole, protect, vmWriteLimiter)

	if cfg.VMMetrics != nil {
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/metrics/history", cfg.VMMetrics)
		mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/metrics/stream", cfg.VMMetrics)
	}
}

// registerAuthRoutes wires the unauthenticated credential-check endpoints
// (per-IP rate limited) and the authenticated token/password endpoints
// (per-user rate limited + CSRF). Extracted from NewRouter for gocyclo.
func registerAuthRoutes(mux *http.ServeMux, cfg RouterConfig, protect protectFunc, authWriteLimiter *userRateLimiter, hops int) {
	// Unauthenticated credential-check endpoints get a per-IP rate limit —
	// nothing else gates repeated guesses against them. The pre-login cluster
	// list and OIDC trigger are also unauthenticated and disclose cluster
	// names, so they share the same limiter to bound enumeration/abuse.
	authLimiter := newIPRateLimiter(authRateLimitMaxRequests, authRateLimitWindow, hops, cfg.Store)
	mux.Handle("POST /api/v1/auth/login", authLimiter.middleware(http.HandlerFunc(cfg.Auth.Login)))
	mux.Handle("POST /api/v1/auth/admin-login", authLimiter.middleware(http.HandlerFunc(cfg.Auth.AdminLogin)))
	mux.HandleFunc("GET /api/v1/auth/me", cfg.Auth.Me)
	mux.Handle("GET /api/v1/auth/clusters", authLimiter.middleware(http.HandlerFunc(cfg.Auth.ServeClusters)))
	mux.Handle("POST /api/v1/auth/oidc", authLimiter.middleware(http.HandlerFunc(cfg.Auth.OIDC)))
	mux.Handle("POST /api/v1/auth/logout", protect(http.HandlerFunc(cfg.Auth.Logout), authWriteLimiter))
	mux.Handle("POST /api/v1/auth/tokens", protect(http.HandlerFunc(cfg.Auth.CreateToken), authWriteLimiter))
	mux.HandleFunc("GET /api/v1/auth/tokens", cfg.Auth.ListTokens)
	mux.Handle("DELETE /api/v1/auth/tokens/{id}", protect(http.HandlerFunc(cfg.Auth.RevokeToken), authWriteLimiter))
	mux.Handle("POST /api/v1/auth/password", protect(http.HandlerFunc(cfg.Auth.ChangePassword), authWriteLimiter))
}

// registerAPINotFound installs the catch-all 404 for unknown /api/ paths across
// every method the SPA might use. Extracted from NewRouter for gocyclo.
func registerAPINotFound(mux *http.ServeMux, cfg RouterConfig) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mux.Handle(method+" /api/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if err := writeError(w, http.StatusNotFound, "unknown API path"); err != nil {
				cfg.Log.Error("failed to write API 404", "component", "httpapi", "error", err)
			}
		}))
	}
}

// registerSPA wires the static SPA handler when WebBuildDir is set. Extracted
// from NewRouter for gocyclo.
func registerSPA(mux *http.ServeMux, cfg RouterConfig) {
	if cfg.WebBuildDir == "" {
		return
	}

	spa := &spaHandler{
		root:  http.Dir(cfg.WebBuildDir),
		index: "/index.html",
		log:   cfg.Log,
	}
	mux.Handle("GET /", spa)
}

// newAdminProtect builds the admin authorization guard used by NewRouter.
// It rejects unauthenticated requests (401), audits and rejects non-admin
// identities (403), and wraps non-GET handlers with the admin write rate
// limiter and CSRF middleware.
func newAdminProtect(cfg RouterConfig, method string, next http.Handler, limiter *userRateLimiter, csrf func(http.Handler) http.Handler, hops int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := cfg.Auth.Principal(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)

			return
		}

		if !identity.IsAdmin {
			if cfg.Store != nil {
				_ = cfg.Store.RecordAdminAction(r.Context(), identity.Username, "auth.admin_denied", "auth", identity.Username,
					`{"summary":"admin route denied to non-admin user","changes":[]}`, clientIP(r, hops))
			}

			writeAuthError(w, http.StatusForbidden, "forbidden", msgAdminOnly)

			return
		}

		if method == http.MethodGet {
			next.ServeHTTP(w, r)

			return
		}

		limiter.middleware(cfg.Auth, csrf(next)).ServeHTTP(w, r)
	})
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

// registerSerialConsoleRoutes wires the serial-terminal endpoints onto mux
// when the handler is present. Extracted from NewRouter to keep its cyclomatic
// complexity under the gocyclo threshold (one if-block per route group adds up
// across console, metrics, admin, tasks, ...).
func registerSerialConsoleRoutes(mux *http.ServeMux, handler *VMSerialConsole, protect func(http.Handler, *userRateLimiter) http.Handler, limiter *userRateLimiter) {
	if handler == nil {
		return
	}

	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/serial-ticket", protect(handler, limiter))
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/serial/websocket", handler)
}
