//nolint:wsl_v5 // template handlers keep cluster selection and catalog mapping adjacent
package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/catalog"
)

// --- Cloud-init template DTOs (T18) ---

type adminCloudInitTemplateDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
	Enabled bool   `json:"enabled"`
}

type cloudInitTemplateCreateRequest struct {
	Cluster string `json:"cluster"`
	Label   string `json:"label"`
	Content string `json:"content"`
}

type cloudInitTemplateUpdateRequest struct {
	Cluster string `json:"cluster"`
	Label   string `json:"label"`
	Content string `json:"content"`
}

// ServeCloudInitTemplates handles GET /api/v1/admin/cloudinit-templates — lists
// every template including disabled ones (unlike T06's catalog reader which
// filters by enabled = 1). Admin-only via the RequireAdmin route guard (T007).
func (h *AdminCatalog) ServeCloudInitTemplates(w http.ResponseWriter, r *http.Request) {
	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	templates, err := catalog.ListCloudInitTemplates(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("admin list cloudinit templates failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	dto := make([]adminCloudInitTemplateDTO, len(templates))
	for i, t := range templates {
		dto[i] = adminCloudInitTemplateDTO{
			ID: t.ID, Label: t.Label, Content: t.Content, Enabled: t.Enabled,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeCloudInitTemplateCreate handles POST /api/v1/admin/cloudinit-templates.
func (h *AdminCatalog) ServeCloudInitTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var req cloudInitTemplateCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	tmpl, err := catalog.CreateCloudInitTemplate(r.Context(), h.store, clusterName, req.Label, req.Content)
	if errors.Is(err, catalog.ErrDuplicateCloudInitTemplate) {
		writeAdminError(w, http.StatusConflict, "duplicate_template", "a template with this label already exists")
		return
	}

	if errors.Is(err, catalog.ErrInvalidCloudInitTemplate) {
		writeAdminError(w, http.StatusBadRequest, "invalid_content", "content must start with #cloud-config")
		return
	}

	if err != nil {
		h.log.Error("admin create cloudinit template failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusCreated, adminCloudInitTemplateDTO{
		ID: tmpl.ID, Label: tmpl.Label, Content: tmpl.Content, Enabled: tmpl.Enabled,
	})
}

// ServeCloudInitTemplateUpdate handles PUT /api/v1/admin/cloudinit-templates/{id}.
func (h *AdminCatalog) ServeCloudInitTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "template id is required")
		return
	}

	var req cloudInitTemplateUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	tmpl, err := catalog.UpdateCloudInitTemplate(r.Context(), h.store, clusterName, id, req.Label, req.Content)
	if errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "cloud-init template \""+id+"\" not found")
		return
	}

	if errors.Is(err, catalog.ErrInvalidCloudInitTemplate) {
		writeAdminError(w, http.StatusBadRequest, "invalid_content", "content must start with #cloud-config")
		return
	}

	if err != nil {
		h.log.Error("admin update cloudinit template failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusOK, adminCloudInitTemplateDTO{
		ID: tmpl.ID, Label: tmpl.Label, Content: tmpl.Content, Enabled: tmpl.Enabled,
	})
}

// ServeCloudInitTemplateDelete handles DELETE /api/v1/admin/cloudinit-templates/{id}.
// The cluster is read from the query string (?cluster=default), matching the
// profile delete handler's convention.
func (h *AdminCatalog) ServeCloudInitTemplateDelete(w http.ResponseWriter, r *http.Request) {
	h.serveCatalogDelete(w, r, "cloud-init template", "template", catalog.DeleteCloudInitTemplate, catalog.ErrCloudInitTemplateNotFound)
}

// ServeCloudInitTemplateToggle handles POST /api/v1/admin/cloudinit-templates/{id}/toggle.
func (h *AdminCatalog) ServeCloudInitTemplateToggle(w http.ResponseWriter, r *http.Request) {
	h.serveCatalogToggle(w, r, "cloud-init template", "template", catalog.SetCloudInitTemplateEnabled, catalog.ErrCloudInitTemplateNotFound)
}
