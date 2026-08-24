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

// Admin docs error codes and messages, centralized so the duplicated literals
// live in one place (go:S1192).
const (
	codeDocInvalidRequest  = "invalid_request"
	codeDocNotFound        = "not_found"
	codeDocDuplicatePage   = "duplicate_page"
	codeDocInvalidPage     = "invalid_page"
	codeDocSystemProtected = "system_protected"

	msgDocInvalidRequestBody = "invalid request body"
	msgDocIDLangRequired     = "documentation id and lang are required"
	msgDocDuplicatePage      = "a documentation page with this title already exists for this language"
	msgDocSystemProtected    = "built-in documentation pages cannot be deleted"

	// maxDocBody reserves 256 bytes of headroom for JSON overhead around a 4 KiB request.
	maxDocBody        = 4*1024 - 256
	msgDocBodyTooLong = "body exceeds the maximum length of 3840 characters"
)

// docNotFoundMsg formats the not-found message for one page id, keeping the
// duplicated string fragments in one place (go:S1192).
func docNotFoundMsg(id string) string {
	return "documentation page \"" + id + "\" not found"
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
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocInvalidRequestBody)
		return
	}

	if len(req.BodyMD) > maxDocBody {
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocBodyTooLong)
		return
	}

	page, err := catalog.CreateDocumentationPage(r.Context(), h.store, req.Title, req.Lang, req.Category, req.BodyMD, req.Audience)
	if errors.Is(err, catalog.ErrDuplicateDocumentationPage) {
		writeAdminError(w, http.StatusConflict, codeDocDuplicatePage, msgDocDuplicatePage)
		return
	}

	if errors.Is(err, catalog.ErrInvalidDocumentationPage) {
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidPage, err.Error())
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
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocIDLangRequired)
		return
	}

	var req docUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocInvalidRequestBody)
		return
	}

	if len(req.BodyMD) > maxDocBody {
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocBodyTooLong)
		return
	}

	page, err := catalog.UpdateDocumentationPage(r.Context(), h.store, id, lang, catalog.DocumentationPageUpdate{
		Title: req.Title, Category: req.Category, BodyMD: req.BodyMD,
		Audience: req.Audience, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		writeAdminError(w, http.StatusNotFound, codeDocNotFound, docNotFoundMsg(id))
		return
	}

	if errors.Is(err, catalog.ErrInvalidDocumentationPage) {
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidPage, err.Error())
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
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocIDLangRequired)
		return
	}

	err := catalog.DeleteDocumentationPage(r.Context(), h.store, id, lang)
	if errors.Is(err, catalog.ErrSystemDocumentationPage) {
		writeAdminError(w, http.StatusForbidden, codeDocSystemProtected, msgDocSystemProtected)
		return
	}

	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		writeAdminError(w, http.StatusNotFound, codeDocNotFound, docNotFoundMsg(id))
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
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocIDLangRequired)
		return
	}

	var req docToggleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, codeDocInvalidRequest, msgDocInvalidRequestBody)
		return
	}

	err := catalog.SetDocumentationPageEnabled(r.Context(), h.store, id, lang, req.Enabled)
	if errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		writeAdminError(w, http.StatusNotFound, codeDocNotFound, docNotFoundMsg(id))
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
