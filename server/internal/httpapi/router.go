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

// NewRouter wires the public API and the static SPA handler.
func NewRouter(health, clusterNodes, clusterRefresh, vms, vmDetail http.Handler, vmCreate *VmCreate, tasks *Tasks, auth *Auth, webBuildDir string, log *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /health", health)
	mux.Handle("GET /api/v1/cluster/nodes", auth.Require(clusterNodes))
	mux.Handle("POST /api/v1/cluster/refresh", auth.Require(clusterRefresh))
	// Not wrapped in auth.Require: the handler needs the resolved Identity
	// itself (for scope enforcement) and calls h.auth.Principal(r) directly,
	// returning 401 on its own — wrapping would just re-run the same check.
	mux.Handle("GET /api/v1/vms", vms)
	// VM creation + catalog + task polling — same Principal pattern as above.
	// Nil guards let router tests omit these handlers without panicking.
	if vmCreate != nil {
		mux.Handle("POST /api/v1/vms", vmCreate)
		mux.Handle("GET /api/v1/vm-create/catalog", http.HandlerFunc(vmCreate.ServeCatalog))
	}
	if tasks != nil {
		mux.Handle("GET /api/v1/tasks/{upid}", tasks)
	}
	// VM detail + actions + delete + patch — all gated by vm.Resolve() inside
	// the handler. Same reason as GET /vms: not wrapped in auth.Require.
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}", vmDetail)
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/actions", vmDetail)
	mux.Handle("DELETE /api/v1/vms/{cluster}/{vmid}", vmDetail)
	mux.Handle("PATCH /api/v1/vms/{cluster}/{vmid}", vmDetail)
	mux.HandleFunc("POST /api/v1/auth/login", auth.Login)
	mux.HandleFunc("POST /api/v1/auth/admin-login", auth.AdminLogin)
	mux.HandleFunc("GET /api/v1/auth/me", auth.Me)
	mux.HandleFunc("POST /api/v1/auth/logout", auth.Logout)
	mux.HandleFunc("POST /api/v1/auth/tokens", auth.CreateToken)
	mux.HandleFunc("GET /api/v1/auth/tokens", auth.ListTokens)
	mux.HandleFunc("DELETE /api/v1/auth/tokens/{id}", auth.RevokeToken)
	mux.HandleFunc("POST /api/v1/auth/password", auth.ChangePassword)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mux.Handle(method+" /api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := writeError(w, http.StatusNotFound, "unknown API path"); err != nil {
				log.Error("failed to write API 404", "component", "httpapi", "error", err)
			}
		}))
	}

	if webBuildDir != "" {
		spa := &spaHandler{
			root:  http.Dir(webBuildDir),
			index: "/index.html",
			log:   log,
		}
		mux.Handle("GET /", spa)
	}

	return mux
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
