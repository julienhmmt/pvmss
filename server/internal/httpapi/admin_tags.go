package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/catalog"
)

// --- Tag DTOs ---

type adminTagDTO struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	VMCount   int    `json:"vmCount"`
	Protected bool   `json:"protected"`
}

type tagCreateRequest struct {
	Cluster string `json:"cluster"`
	Name    string `json:"name"`
	Color   string `json:"color"`
}

type tagColorRequest struct {
	Cluster string `json:"cluster"`
	Color   string `json:"color"`
}

// ServeTags handles GET /api/v1/admin/tags — lists all tags with live VM
// counts computed from the inventory projection (FR-015).
func (h *AdminCatalog) ServeTags(w http.ResponseWriter, r *http.Request) {
	clusterName := queryCluster(r)

	tags, err := catalog.ListTags(r.Context(), h.store, h.projection, clusterName)
	if err != nil {
		h.log.Error("admin list tags failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	dto := make([]adminTagDTO, len(tags))
	for i, t := range tags {
		dto[i] = adminTagDTO{
			Name: t.Name, Color: t.Color, VMCount: t.VMCount, Protected: t.Protected,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeTagCreate handles POST /api/v1/admin/tags.
func (h *AdminCatalog) ServeTagCreate(w http.ResponseWriter, r *http.Request) {
	var req tagCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	clusterName := resolveCluster(req.Cluster)

	tag, err := catalog.CreateTag(r.Context(), h.store, clusterName, req.Name, req.Color)
	if errors.Is(err, catalog.ErrDuplicateTag) {
		writeAdminError(w, http.StatusConflict, "duplicate_tag", "tag \""+req.Name+"\" already exists")
		return
	}

	if errors.Is(err, catalog.ErrInvalidTagName) {
		writeAdminError(w, http.StatusBadRequest, "invalid_tag_name", err.Error())
		return
	}

	if err != nil {
		h.log.Error("admin create tag failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	writeAdminJSON(w, http.StatusCreated, adminTagDTO{
		Name: tag.Name, Color: tag.Color, VMCount: h.tagVMCount(tag.Name), Protected: tag.Protected,
	})
}

// ServeTagColor handles PUT /api/v1/admin/tags/{name}/color.
func (h *AdminCatalog) ServeTagColor(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "tag name is required")
		return
	}

	var req tagColorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	clusterName := resolveCluster(req.Cluster)

	tag, err := catalog.SetTagColor(r.Context(), h.store, clusterName, name, req.Color)
	if errors.Is(err, catalog.ErrTagNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "tag \""+name+"\" not found")
		return
	}

	if errors.Is(err, catalog.ErrInvalidTagColor) {
		writeAdminError(w, http.StatusBadRequest, "invalid_tag_color", err.Error())
		return
	}

	if errors.Is(err, catalog.ErrInvalidTagName) {
		writeAdminError(w, http.StatusBadRequest, "invalid_tag_name", err.Error())
		return
	}

	if err != nil {
		h.log.Error("admin update tag color failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	writeAdminJSON(w, http.StatusOK, adminTagDTO{
		Name: tag.Name, Color: tag.Color, VMCount: h.tagVMCount(tag.Name), Protected: tag.Protected,
	})
}

// ServeTagDelete handles DELETE /api/v1/admin/tags/{name}. The cluster is
// read from the query string (?cluster=default), not the JSON body —
// DELETE-with-body is awkward and the frontend uses the query param form.
func (h *AdminCatalog) ServeTagDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "tag name is required")
		return
	}

	clusterName := queryCluster(r)

	err := catalog.DeleteTag(r.Context(), h.store, clusterName, name)
	if errors.Is(err, catalog.ErrProtectedTag) {
		writeAdminError(w, http.StatusForbidden, "protected_tag", "the pvmss tag cannot be deleted")
		return
	}

	if errors.Is(err, catalog.ErrTagNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "tag \""+name+"\" not found")
		return
	}

	if err != nil {
		h.log.Error("admin delete tag failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	writeAdminJSON(w, http.StatusOK, statusResponse{Status: statusDeleted})
}

// tagVMCount returns the live count of VMs tagged with name, computed from the
// inventory projection (FR-015: never stored). Returns 0 when the projection
// is nil (tests that don't exercise tags).
func (h *AdminCatalog) tagVMCount(name string) int {
	if h.projection == nil {
		return 0
	}

	idx := h.projection.Load()
	if idx == nil {
		return 0
	}

	count := 0

	for _, vm := range idx.ByVMID {
		for _, t := range vm.Tags {
			if t == name {
				count++
			}
		}
	}

	return count
}
