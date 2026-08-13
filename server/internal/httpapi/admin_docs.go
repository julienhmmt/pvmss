//nolint:wsl_v5 // docs admin handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/store"
)

// AdminDocs serves the admin documentation CRUD endpoints (issue #53): list
// all pages (all langs, enabled+disabled), create, update, delete (refuses
// system pages), and toggle. Every route is wrapped by Auth.RequireAdmin at
// the router (FR-008), the same guard as every other admin surface.
type AdminDocs struct {
	auth  *Auth
	store *store.Store
	docs  *DocsAPIHandler
	log   *slog.Logger
}

// NewAdminDocs creates the admin docs handler. The DocsAPIHandler is shared so
// admin mutations can invalidate the public render cache.
func NewAdminDocs(authHandler *Auth, st *store.Store, docs *DocsAPIHandler, log *slog.Logger) *AdminDocs {
	return &AdminDocs{auth: authHandler, store: st, docs: docs, log: log}
}

// adminDocDTO is the admin response shape for one page (full content).
type adminDocDTO struct {
	ID        string `json:"id"`
	Lang      string `json:"lang"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	BodyMD    string `json:"bodyMd"`
	Audience  string `json:"audience"`
	Enabled   bool   `json:"enabled"`
	IsSystem  bool   `json:"isSystem"`
	SortOrder int    `json:"sortOrder"`
}

type docCreateRequest struct {
	Title    string `json:"title"`
	Lang     string `json:"lang"`
	Category string `json:"category"`
	BodyMD   string `json:"bodyMd"`
	Audience string `json:"audience"`
}

type docUpdateRequest struct {
	Title     string `json:"title"`
	Lang      string `json:"lang"`
	Category  string `json:"category"`
	BodyMD    string `json:"bodyMd"`
	Audience  string `json:"audience"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sortOrder"`
}

type docToggleRequest struct {
	Enabled bool `json:"enabled"`
}

func toAdminDocDTO(p catalog.DocumentationPage) adminDocDTO {
	return adminDocDTO{
		ID: p.ID, Lang: p.Lang, Title: p.Title, Category: p.Category, BodyMD: p.BodyMD,
		Audience: p.Audience, Enabled: p.Enabled, IsSystem: p.IsSystem, SortOrder: p.SortOrder,
	}
}

// ServeDocsList handles GET /api/v1/admin/docs — every page (all langs,
// enabled+disabled), ordered by sort_order then title.
func (h *AdminDocs) ServeDocsList(w http.ResponseWriter, r *http.Request) {
	pages, err := catalog.ListDocumentationPages(r.Context(), h.store)
	if err != nil {
		h.log.Error("admin list docs failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}

	dto := make([]adminDocDTO, len(pages))
	for i, p := range pages {
		dto[i] = toAdminDocDTO(p)
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeDocCreate handles POST /api/v1/admin/docs.
func (h *AdminDocs) ServeDocCreate(w http.ResponseWriter, r *http.Request) {
	var req docCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	page, err := catalog.CreateDocumentationPage(r.Context(), h.store, req.Title, req.Lang, req.Category, req.BodyMD, req.Audience)
	if errors.Is(err, catalog.ErrDuplicateDocumentationPage) {
		writeAdminError(w, http.StatusConflict, "duplicate_page", "a documentation page with this title already exists for this language")
		return
	}

	if errors.Is(err, catalog.ErrInvalidDocumentationPage) {
		writeAdminError(w, http.StatusBadRequest, "invalid_page", err.Error())
		return
	}

	if err != nil {
		h.log.Error("admin create doc failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}

	h.docs.InvalidateDocCache()
	writeAdminJSON(w, http.StatusCreated, toAdminDocDTO(page))
}

// ServeDocUpdate handles PUT /api/v1/admin/docs/{id}/{lang}.
func (h *AdminDocs) ServeDocUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lang := r.PathValue("lang")
	if id == "" || lang == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "documentation id and lang are required")
		return
	}

	var req docUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	page, err := catalog.UpdateDocumentationPage(r.Context(), h.store, id, lang, req.Title, req.Category, req.BodyMD, req.Audience, req.Enabled, req.SortOrder)
	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "documentation page \""+id+"\" not found")
		return
	}

	if errors.Is(err, catalog.ErrInvalidDocumentationPage) {
		writeAdminError(w, http.StatusBadRequest, "invalid_page", err.Error())
		return
	}

	if err != nil {
		h.log.Error("admin update doc failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}

	h.docs.InvalidateDocCache()
	writeAdminJSON(w, http.StatusOK, toAdminDocDTO(page))
}

// ServeDocDelete handles DELETE /api/v1/admin/docs/{id}/{lang}. Refuses
// built-in system pages with 403.
func (h *AdminDocs) ServeDocDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lang := r.PathValue("lang")
	if id == "" || lang == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "documentation id and lang are required")
		return
	}

	err := catalog.DeleteDocumentationPage(r.Context(), h.store, id, lang)
	if errors.Is(err, catalog.ErrSystemDocumentationPage) {
		writeAdminError(w, http.StatusForbidden, "system_protected", "built-in documentation pages cannot be deleted")
		return
	}

	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "documentation page \""+id+"\" not found")
		return
	}

	if err != nil {
		h.log.Error("admin delete doc failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}

	h.docs.InvalidateDocCache()
	writeAdminJSON(w, http.StatusOK, statusResponse{Status: statusDeleted})
}

// ServeDocToggle handles POST /api/v1/admin/docs/{id}/{lang}/toggle.
func (h *AdminDocs) ServeDocToggle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lang := r.PathValue("lang")
	if id == "" || lang == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "documentation id and lang are required")
		return
	}

	var req docToggleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	err := catalog.SetDocumentationPageEnabled(r.Context(), h.store, id, lang, req.Enabled)
	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "documentation page \""+id+"\" not found")
		return
	}

	if err != nil {
		h.log.Error("admin toggle doc failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}

	h.docs.InvalidateDocCache()
	writeAdminJSON(w, http.StatusOK, docToggleResponse{ID: id, Lang: lang, Enabled: req.Enabled})
}

// docToggleResponse is the toggle response shape for docs (carries lang too).
type docToggleResponse struct {
	ID      string `json:"id"`
	Lang    string `json:"lang"`
	Enabled bool   `json:"enabled"`
}
