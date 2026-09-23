//nolint:wsl_v5 // publish handlers keep cluster selection and response mapping adjacent
package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"time"
)

// adminNodePublicationDTO is one node's publication outcome.
type adminNodePublicationDTO struct {
	Node  string `json:"node"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// adminPublicationDTO is the latest publication of one document.
type adminPublicationDTO struct {
	Filename    string                    `json:"filename"`
	PublishedAt string                    `json:"publishedAt"`
	Nodes       []adminNodePublicationDTO `json:"nodes"`
}

func publicationDTO(p store.CloudInitPublication) adminPublicationDTO {
	nodes := make([]adminNodePublicationDTO, len(p.Nodes))
	for i, n := range p.Nodes {
		nodes[i] = adminNodePublicationDTO{Node: n.Node, OK: n.OK, Error: n.Error}
	}

	return adminPublicationDTO{Filename: p.Filename, PublishedAt: p.PublishedAt.Format(time.RFC3339Nano), Nodes: nodes}
}

// adminPublishAllDTO is the response of the resync endpoint.
type adminPublishAllDTO struct {
	Publications map[string]adminPublicationDTO `json:"publications"`
	Error        string                         `json:"error,omitempty"`
}

// publisherFor resolves the cluster's snippet publisher. ok=false when the
// cluster client cannot publish at all.
func (h *AdminCatalog) publisherFor(clusterName string) (cluster.SnippetPublisher, bool) {
	client, err := h.clientFor(clusterName)
	if err != nil {
		return nil, false
	}

	publisher, ok := client.(cluster.SnippetPublisher)

	return publisher, ok
}

// publishTemplate publishes one template after an admin create/update/enable.
// Publishing is best-effort from the admin action's point of view: the
// template row is saved either way, and the returned publication (or error
// message) tells the admin where it landed.
func (h *AdminCatalog) publishTemplate(ctx context.Context, clusterName string, tmpl catalog.CloudInitTemplate) (*adminPublicationDTO, string) {
	if !tmpl.Enabled {
		return nil, ""
	}

	publisher, ok := h.publisherFor(clusterName)
	if !ok || !publisher.PublishingEnabled() {
		return nil, cluster.ErrSnippetWriteUnavailable.Error()
	}

	publication, err := catalog.PublishCloudInitDocument(ctx, h.store, publisher, clusterName, tmpl.ID, tmpl.Content)
	if err != nil {
		h.log.Error("publish cloud-init template failed", "component", "httpapi", "cluster", clusterName, "template", tmpl.ID, "error", err)

		return nil, err.Error()
	}

	dto := publicationDTO(publication)

	return &dto, ""
}

// ServeCloudInitPublishAll handles POST /api/v1/admin/cloudinit-templates/publish
// ?cluster=<name>: republishes the baseline and every enabled template to
// every node (after adding a node, reinstalling one, or upgrading PVMSS).
func (h *AdminCatalog) ServeCloudInitPublishAll(w http.ResponseWriter, r *http.Request) {
	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	publisher, ok := h.publisherFor(clusterName)
	if !ok || !publisher.PublishingEnabled() {
		writeAdminError(w, http.StatusConflict, "cloudinit_write_unavailable", cluster.ErrSnippetWriteUnavailable.Error())
		return
	}

	publications, err := catalog.PublishAllCloudInitDocuments(r.Context(), h.store, publisher, clusterName)
	if err != nil && len(publications) == 0 {
		h.log.Error("publish all cloud-init documents failed", "component", "httpapi", "cluster", clusterName, "error", err)
		writeAdminError(w, http.StatusBadGateway, "publish_failed", err.Error())
		return
	}

	dto := adminPublishAllDTO{Publications: make(map[string]adminPublicationDTO, len(publications))}
	for _, p := range publications {
		dto.Publications[p.TemplateID] = publicationDTO(p)
	}

	if err != nil {
		dto.Error = err.Error()
	}

	h.recordAdminAction(r, "admin.cloudinit_templates.publish", "cloudinit_template", "*",
		fmt.Sprintf("published %d cloud-init documents on cluster %s", len(publications), clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName}})
	writeAdminJSON(w, http.StatusOK, dto)
}

// publicationsFor loads every publication of the cluster for the list view.
func (h *AdminCatalog) publicationsFor(ctx context.Context, clusterName string) map[string]store.CloudInitPublication {
	publications, err := h.store.ListCloudInitPublications(ctx, clusterName)
	if err != nil {
		h.log.Error("list cloud-init publications failed", "component", "httpapi", "cluster", clusterName, "error", err)

		return nil
	}

	return publications
}
