package httpapi

import (
	"encoding/json"
	"errors"
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

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeError writes a JSON error response with the given status code and detail.
func writeError(w http.ResponseWriter, status int, detail string) {
	body, _ := json.Marshal(errorResponse{Detail: detail})
	writeJSON(w, status, body)
}

// NewRouter wires the public API and the static SPA handler.
func NewRouter(health http.Handler, webBuildDir string, log *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /health", health)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mux.Handle(method+" /api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "unknown API path")
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

// spaHandler serves static assets and falls back to index.html for client-side routes.
type spaHandler struct {
	root  http.FileSystem
	index string
	log   *slog.Logger
}

func (s *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(r.URL.Path)
	if p == "." {
		p = "/"
	}

	// Asset paths and files with an extension must exist; never fall back.
	if strings.HasPrefix(p, "/_app/") || path.Ext(p) != "" {
		if err := s.serveFile(w, r, p); err != nil {
			writeError(w, http.StatusNotFound, "asset not found")
		}
		return
	}

	// For client-side routes, try the file first, then fall back to index.html.
	if err := s.serveFile(w, r, p); err == nil {
		return
	}
	if err := s.serveFile(w, r, s.index); err != nil {
		writeError(w, http.StatusNotFound, "shell not found")
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
	http.ServeContent(w, r, name, st.ModTime(), f)
	return nil
}
