//nolint:wsl_v5 // template handlers keep cluster selection and catalog mapping adjacent
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/store"
)

// maxCloudInitTemplateBody is the explicit server-side content cap for an
// admin cloud-init template. It matches cloudinit.MaxSnippetSize so admin
// templates and VM snippets share the same boundary.
const maxCloudInitTemplateBody = 16 * 1024

//  - Cloud-init template DTOs -

type adminCloudInitTemplateDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
	Enabled bool   `json:"enabled"`
	// Publication is the template's latest publication (nil: never
	// published). PublishError is set when this request tried to publish
	// and could not even start (not configured, nodes unreadable); per-node
	// failures are in Publication.Nodes.
	Publication  *adminPublicationDTO `json:"publication"`
	PublishError string               `json:"publishError,omitempty"`
}

func templateDTO(t catalog.CloudInitTemplate) adminCloudInitTemplateDTO {
	return adminCloudInitTemplateDTO{ID: t.ID, Label: t.Label, Content: t.Content, Enabled: t.Enabled}
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

// ServeCloudInitTemplates handles GET /api/v1/admin/cloudinit-templates - lists
// every template including disabled ones (unlike catalog reader which filters by enabled = 1).
// Admin-only via the RequireAdmin route guard.
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

	publications := h.publicationsFor(r.Context(), clusterName)

	dto := make([]adminCloudInitTemplateDTO, len(templates))
	for i, t := range templates {
		dto[i] = templateDTO(t)

		if p, ok := publications[t.ID]; ok {
			pub := publicationDTO(p)
			dto[i].Publication = &pub
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeCloudInitTemplateCreate handles POST /api/v1/admin/cloudinit-templates.
func (h *AdminCatalog) ServeCloudInitTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var req cloudInitTemplateCreateRequest
	if err := decodeJSONLimit(w, r, &req, maxCloudInitTemplateBody+4*1024); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if len(req.Content) > maxCloudInitTemplateBody {
		writeAdminError(w, http.StatusBadRequest, "invalid_content", "content exceeds the maximum size of 16 KiB")
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

	h.recordAdminAction(r, "admin.cloudinit_templates.create", "cloudinit_template", tmpl.ID,
		fmt.Sprintf("created cloud-init template %s (%s) on cluster %s", tmpl.Label, tmpl.ID, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "id": tmpl.ID, auditKeyLabel: tmpl.Label, auditKeyEnabled: tmpl.Enabled}})
	dto := templateDTO(tmpl)
	dto.Publication, dto.PublishError = h.publishTemplate(r.Context(), clusterName, tmpl)
	writeAdminJSON(w, http.StatusCreated, dto)
}

// ServeCloudInitTemplateUpdate handles PUT /api/v1/admin/cloudinit-templates/{id}.
func (h *AdminCatalog) ServeCloudInitTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "template id is required")
		return
	}

	var req cloudInitTemplateUpdateRequest
	if err := decodeJSONLimit(w, r, &req, maxCloudInitTemplateBody+4*1024); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if len(req.Content) > maxCloudInitTemplateBody {
		writeAdminError(w, http.StatusBadRequest, "invalid_content", "content exceeds the maximum size of 16 KiB")
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

	h.recordAdminAction(r, "admin.cloudinit_templates.update", "cloudinit_template", id,
		fmt.Sprintf("updated cloud-init template %s (%s) on cluster %s", tmpl.Label, id, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "id": id, auditKeyLabel: tmpl.Label, auditKeyEnabled: tmpl.Enabled}})
	dto := templateDTO(tmpl)
	dto.Publication, dto.PublishError = h.publishTemplate(r.Context(), clusterName, tmpl)
	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeCloudInitTemplateDelete handles DELETE /api/v1/admin/cloudinit-templates/{id}.
// The cluster is read from the query string (?cluster=default), matching the
// profile delete handler's convention.
func (h *AdminCatalog) ServeCloudInitTemplateDelete(w http.ResponseWriter, r *http.Request) {
	h.serveCatalogDelete(w, r, "cloud-init template", "template", catalog.DeleteCloudInitTemplate, catalog.ErrCloudInitTemplateNotFound)
}

// ServeCloudInitTemplateToggle handles POST /api/v1/admin/cloudinit-templates/{id}/toggle.
// Enabling a template also publishes it (a template disabled before the
// first publish would otherwise be offered to users with no file behind it).
func (h *AdminCatalog) ServeCloudInitTemplateToggle(w http.ResponseWriter, r *http.Request) {
	publishOnEnable := func(ctx context.Context, st *store.Store, clusterName, id string, enabled bool) error {
		if err := catalog.SetCloudInitTemplateEnabled(ctx, st, clusterName, id, enabled); err != nil {
			return err
		}

		if !enabled {
			return nil
		}

		tmpl, err := catalog.FindCloudInitTemplate(ctx, st, clusterName, id)
		if err != nil {
			return err
		}

		if _, msg := h.publishTemplate(ctx, clusterName, tmpl); msg != "" {
			h.log.Warn("cloud-init template enabled but not published", "component", "httpapi", "cluster", clusterName, "template", id, "error", msg)
		}

		return nil
	}

	h.serveCatalogToggle(w, r, "cloud-init template", "template", publishOnEnable, catalog.ErrCloudInitTemplateNotFound)
}
