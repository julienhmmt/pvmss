package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/store"
)

// CloudInitFiles serves the user-owned cloud-init document endpoints
// (/api/v1/cloudinit/files — cloudinit-userdata spec D3). Every row is
// scoped to the session identity's username; there is no cluster or admin
// concept here.
type CloudInitFiles struct {
	auth  *Auth
	store *store.Store
	log   *slog.Logger
}

// NewCloudInitFiles builds the handler; wired in main.go like the other
// optional handlers.
func NewCloudInitFiles(authHandler *Auth, st *store.Store, log *slog.Logger) *CloudInitFiles {
	return &CloudInitFiles{auth: authHandler, store: st, log: log}
}

type cloudInitFileListDTO struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	UpdatedAt string `json:"updatedAt"`
}

type cloudInitFileDTO struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type cloudInitFileBody struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

func toCloudInitFileDTO(f store.UserCloudInitFile) cloudInitFileDTO {
	return cloudInitFileDTO{
		ID: f.ID, Label: f.Label, Content: f.Content,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

// ServeList handles GET /api/v1/cloudinit/files.
func (h *CloudInitFiles) ServeList(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.fileOwner(w, r)
	if !ok {
		return
	}

	files, err := cloudinit.ListUserFiles(r.Context(), h.store, owner)
	if err != nil {
		h.fail(w, "list cloud-init files", err)

		return
	}

	out := struct {
		Files []cloudInitFileListDTO `json:"files"`
	}{Files: make([]cloudInitFileListDTO, 0, len(files))}

	for _, f := range files {
		out.Files = append(out.Files, cloudInitFileListDTO{ID: f.ID, Label: f.Label, UpdatedAt: f.UpdatedAt})
	}

	h.writeJSON(w, http.StatusOK, out)
}

// ServeGet handles GET /api/v1/cloudinit/files/{id}.
func (h *CloudInitFiles) ServeGet(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.fileOwner(w, r)
	if !ok {
		return
	}

	f, err := cloudinit.GetUserFile(r.Context(), h.store, owner, r.PathValue("id"))
	if err != nil {
		h.fail(w, "get cloud-init file", err)

		return
	}

	h.writeJSON(w, http.StatusOK, toCloudInitFileDTO(f))
}

// ServeCreate handles POST /api/v1/cloudinit/files.
func (h *CloudInitFiles) ServeCreate(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.fileOwner(w, r)
	if !ok {
		return
	}

	var req cloudInitFileBody
	if err := decodeJSONLimit(w, r, &req, maxCloudInitTemplateBody+4*1024); err != nil {
		h.writeErr(w, http.StatusBadRequest, "invalid_request", "invalid request body")

		return
	}

	f, err := cloudinit.CreateUserFile(r.Context(), h.store, owner, req.Label, req.Content)
	if err != nil {
		h.fail(w, "create cloud-init file", err)

		return
	}

	h.writeJSON(w, http.StatusCreated, toCloudInitFileDTO(f))
}

// ServeUpdate handles PUT /api/v1/cloudinit/files/{id}.
func (h *CloudInitFiles) ServeUpdate(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.fileOwner(w, r)
	if !ok {
		return
	}

	var req cloudInitFileBody
	if err := decodeJSONLimit(w, r, &req, maxCloudInitTemplateBody+4*1024); err != nil {
		h.writeErr(w, http.StatusBadRequest, "invalid_request", "invalid request body")

		return
	}

	f, err := cloudinit.UpdateUserFile(r.Context(), h.store, owner, r.PathValue("id"), req.Label, req.Content)
	if err != nil {
		h.fail(w, "update cloud-init file", err)

		return
	}

	h.writeJSON(w, http.StatusOK, toCloudInitFileDTO(f))
}

// ServeDelete handles DELETE /api/v1/cloudinit/files/{id}.
func (h *CloudInitFiles) ServeDelete(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.fileOwner(w, r)
	if !ok {
		return
	}

	if err := cloudinit.DeleteUserFile(r.Context(), h.store, owner, r.PathValue("id")); err != nil {
		h.fail(w, "delete cloud-init file", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// fileOwner resolves the session identity — the only owner this API ever
// operates on. 401 when unauthenticated.
func (h *CloudInitFiles) fileOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeErr(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)

		return "", false
	}

	return identity.Username, true
}

// fail maps a domain sentinel to its HTTP response. Anything unrecognized is
// a 500 logged with the operation name.
func (h *CloudInitFiles) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, cloudinit.ErrUserFileInvalid):
		h.writeErr(w, http.StatusBadRequest, "invalid_cloudinit_file", err.Error())
	case errors.Is(err, cloudinit.ErrUserFileDuplicate):
		h.writeErr(w, http.StatusConflict, "duplicate_cloudinit_file", "a file with this label already exists")
	case errors.Is(err, cloudinit.ErrUserFileLimit):
		h.writeErr(w, http.StatusConflict, "cloudinit_file_limit", "you can keep at most 20 cloud-init files")
	case errors.Is(err, cloudinit.ErrUserFileNotFound):
		h.writeErr(w, http.StatusNotFound, "not_found", err.Error())
	default:
		h.log.Error("cloud-init files: "+op+" failed", "component", "httpapi", "error", err)
		h.writeErr(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
	}
}

func (h *CloudInitFiles) writeErr(w http.ResponseWriter, status int, code, message string) {
	body, err := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
	if err != nil {
		h.log.Error("failed to marshal cloud-init file error", "component", "httpapi", "error", err)
		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write cloud-init file error", "component", "httpapi", "error", err)
	}
}

func (h *CloudInitFiles) writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal cloud-init file response", "component", "httpapi", "error", err)
		h.writeErr(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write cloud-init file response", "component", "httpapi", "error", err)
	}
}
