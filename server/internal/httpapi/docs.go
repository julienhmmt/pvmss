//nolint:wsl_v5 // docs handlers keep cache invalidation and catalog mapping adjacent
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/store"
	"sync"
)

// DocsAPIHandler serves the public documentation read endpoints (issue #53):
// the audience-filtered page list and the rendered-HTML single-page view. The
// admin CRUD endpoints live on AdminDocs (admin_docs.go) behind RequireAdmin.
type DocsAPIHandler struct {
	auth  *Auth
	store *store.Store
	log   *slog.Logger

	cacheMu sync.RWMutex
	cache   map[docCacheKey]string
}

type docCacheKey struct {
	id   string
	lang string
}

// NewDocsAPIHandler creates the public docs handler. The store is the same
// *store.Store shared with the admin handler; admin mutations call
// InvalidateDocCache to keep the rendered-HTML cache consistent.
func NewDocsAPIHandler(authHandler *Auth, st *store.Store, log *slog.Logger) *DocsAPIHandler {
	return &DocsAPIHandler{auth: authHandler, store: st, log: log, cache: make(map[docCacheKey]string)}
}

// InvalidateDocCache drops the entire rendered-HTML cache. Called by every
// admin mutation (create/update/delete/toggle) so the next read re-renders.
func (h *DocsAPIHandler) InvalidateDocCache() {
	h.cacheMu.Lock()
	h.cache = make(map[docCacheKey]string)
	h.cacheMu.Unlock()
}

// docSummaryDTO is one entry in the public list response.
type docSummaryDTO struct {
	ID       string `json:"id"`
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Audience string `json:"audience"`
}

// docRenderedDTO is the single-page response: the rendered HTML only.
type docRenderedDTO struct {
	ID    string `json:"id"`
	Lang  string `json:"lang"`
	Title string `json:"title"`
	HTML  string `json:"html"`
}

// ServeDocsList handles GET /api/v1/docs — the audience-filtered list of
// enabled pages. Admin-audience pages are hidden unless the caller is an admin.
func (h *DocsAPIHandler) ServeDocsList(w http.ResponseWriter, r *http.Request) {
	pages, err := catalog.EnabledDocumentationPages(r.Context(), h.store)
	if err != nil {
		h.log.Error("docs list failed", "component", "httpapi", "error", err)
		_ = writeError(w, http.StatusInternalServerError, msgInternalServerError)
		return
	}

	isAdmin := h.callerIsAdmin(r)

	dto := make([]docSummaryDTO, 0, len(pages))
	for _, p := range pages {
		if p.Audience == catalog.DocumentationAudienceAdmin && !isAdmin {
			continue
		}

		dto = append(dto, docSummaryDTO{
			ID: p.ID, Lang: p.Lang, Title: p.Title, Category: p.Category, Audience: p.Audience,
		})
	}

	writeJSON2(w, http.StatusOK, dto)
}

// ServeDoc handles GET /api/v1/docs/{id}?lang= — the rendered-HTML single-page
// view. Resolution uses catalog.GetDocumentationPage (en fallback). Unknown id
// → 404; admin-audience page with non-admin caller → 403; missing JWT on an
// admin page → 401.
func (h *DocsAPIHandler) ServeDoc(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		_ = writeError(w, http.StatusBadRequest, "documentation id is required")
		return
	}

	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}

	page, err := catalog.GetDocumentationPage(r.Context(), h.store, id, lang)
	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		_ = writeError(w, http.StatusNotFound, "documentation page \""+id+"\" not found")
		return
	}

	if err != nil {
		h.log.Error("docs get failed", "component", "httpapi", "error", err)
		_ = writeError(w, http.StatusInternalServerError, msgInternalServerError)
		return
	}

	// Disabled pages are never served to the public, even if the id is known.
	if !page.Enabled {
		_ = writeError(w, http.StatusNotFound, "documentation page \""+id+"\" not found")
		return
	}

	if page.Audience == catalog.DocumentationAudienceAdmin {
		identity, authErr := h.auth.Principal(r)
		if authErr != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}

		if !identity.IsAdmin {
			writeAuthError(w, http.StatusForbidden, "forbidden", "admin only")
			return
		}
	}

	htmlBody := h.renderedHTML(id, page.Lang, page.BodyMD)
	writeJSON2(w, http.StatusOK, docRenderedDTO{ID: page.ID, Lang: page.Lang, Title: page.Title, HTML: htmlBody})
}

// renderedHTML returns the cached rendered HTML for (id, lang), rendering and
// caching on miss. The cache is invalidated by admin mutations.
func (h *DocsAPIHandler) renderedHTML(id, lang, bodyMD string) string {
	key := docCacheKey{id: id, lang: lang}

	h.cacheMu.RLock()
	if cached, ok := h.cache[key]; ok {
		h.cacheMu.RUnlock()
		return cached
	}
	h.cacheMu.RUnlock()

	rendered := renderMarkdownToHTML(bodyMD)

	h.cacheMu.Lock()
	h.cache[key] = rendered
	h.cacheMu.Unlock()

	return rendered
}

// callerIsAdmin resolves the caller and reports IsAdmin, returning false when
// unauthenticated (the public list simply hides admin pages for anonymous
// callers — it never 401s).
func (h *DocsAPIHandler) callerIsAdmin(r *http.Request) bool {
	identity, err := h.auth.Principal(r)
	if err != nil {
		return false
	}

	return identity.IsAdmin
}

// writeJSON2 marshals value and writes it with the given status. The public
// docs endpoints use the plain errorResponse shape (via writeError) on
// failure, not the admin error envelope, so this stays separate from
// writeAdminJSON.
func writeJSON2(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		_ = writeError(w, http.StatusInternalServerError, msgInternalServerError)
		return
	}

	_ = writeJSON(w, status, body)
}
