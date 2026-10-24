//nolint:wsl_v5 // template handlers keep cluster selection and catalog mapping adjacent
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cloudinit"
	"strings"
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
	// Document is the file to paste on the nodes and where it is, read
	// live (nil: not checked - disabled template, or DocumentError).
	Document      *adminDocumentDTO `json:"document"`
	DocumentError string            `json:"documentError,omitempty"`
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
		SetErrorMsg(w, "admin list cloudinit templates failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	dto := make([]adminCloudInitTemplateDTO, len(templates))
	for i, t := range templates {
		dto[i] = h.templateWithDocument(r.Context(), clusterName, t)
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
		code, message := cloudInitTemplateError(err)
		writeAdminError(w, http.StatusBadRequest, code, message)

		return
	}

	if err != nil {
		SetErrorMsg(w, "admin create cloudinit template failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.cloudinit_templates.create", "cloudinit_template", tmpl.ID,
		fmt.Sprintf("created cloud-init template %s (%s) on cluster %s", tmpl.Label, tmpl.ID, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "id": tmpl.ID, auditKeyLabel: tmpl.Label, auditKeyEnabled: tmpl.Enabled}})
	writeAdminJSON(w, http.StatusCreated, h.templateWithDocument(r.Context(), clusterName, tmpl))
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
		code, message := cloudInitTemplateError(err)
		writeAdminError(w, http.StatusBadRequest, code, message)

		return
	}

	if err != nil {
		SetErrorMsg(w, "admin update cloudinit template failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.cloudinit_templates.update", "cloudinit_template", id,
		fmt.Sprintf("updated cloud-init template %s (%s) on cluster %s", tmpl.Label, id, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "id": id, auditKeyLabel: tmpl.Label, auditKeyEnabled: tmpl.Enabled}})
	writeAdminJSON(w, http.StatusOK, h.templateWithDocument(r.Context(), clusterName, tmpl))
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

// templateWithDocument maps a template and, when enabled, checks its file on
// the nodes.
func (h *AdminCatalog) templateWithDocument(ctx context.Context, clusterName string, t catalog.CloudInitTemplate) adminCloudInitTemplateDTO {
	dto := templateDTO(t)
	if !t.Enabled {
		return dto
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		dto.DocumentError = err.Error()

		return dto
	}

	dto.Document, dto.DocumentError = documentFor(ctx, client, t.ID, t.Content)

	return dto
}

// cloudInitTemplateError maps a template validation failure (create and
// update) to its code and message: a YAML parse error keeps the parser's
// line, a missing header or label says so.
func cloudInitTemplateError(err error) (code, message string) {
	switch {
	case errors.Is(err, cloudinit.ErrSnippetInvalidYAML):
		return "invalid_yaml", strings.TrimPrefix(err.Error(), catalog.ErrInvalidCloudInitTemplate.Error()+": ")
	case errors.Is(err, cloudinit.ErrSnippetPrefix):
		return "invalid_content", "content must start with #cloud-config"
	default:
		return "invalid_content", strings.TrimPrefix(err.Error(), catalog.ErrInvalidCloudInitTemplate.Error()+": ")
	}
}
