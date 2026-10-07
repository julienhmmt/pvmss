//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"strconv"
)

// adminTemplateDTO is the admin response shape for one discovered template.
// missing is true for a stored approval whose template Proxmox no longer
// reports - the UI offers Remove on those rows only.
type adminTemplateDTO struct {
	VMID              int    `json:"vmid"`
	Node              string `json:"node"`
	Name              string `json:"name"`
	CloudInitCapable  bool   `json:"cloudInitCapable"`
	DiskStorage       string `json:"diskStorage"`
	DiskSizeGB        int    `json:"diskSizeGB"`
	DiskBus           string `json:"diskBus"`
	Enabled           bool   `json:"enabled"`
	Missing           bool   `json:"missing"`
	DiskUnreadable    bool   `json:"diskUnreadable"`
	OverrideDiscovery bool   `json:"overrideDiscovery"`
}

// ServeTemplates handles GET /api/v1/admin/templates.
//
//nolint:dupl // structurally similar to ServeNodes by design (row→DTO mapping)
func (h *AdminCatalog) ServeTemplates(w http.ResponseWriter, r *http.Request) {
	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return
	}

	templates, err := catalog.AdminListTemplates(r.Context(), h.store, client, clusterName)
	if err != nil {
		writeAdminFailure(w, "admin list templates failed", err)

		return
	}

	dto := make([]adminTemplateDTO, len(templates))
	for i, tmpl := range templates {
		dto[i] = adminTemplateDTO{
			VMID: tmpl.VMID, Node: tmpl.Node, Name: tmpl.Name,
			CloudInitCapable: tmpl.CloudInitCapable, DiskStorage: tmpl.DiskStorage,
			DiskSizeGB: tmpl.DiskSizeGB, DiskBus: tmpl.DiskBus, Enabled: tmpl.Enabled,
			Missing: tmpl.Missing, DiskUnreadable: tmpl.DiskUnreadable,
			OverrideDiscovery: tmpl.OverrideDiscovery,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type templateToggleRequest struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Enabled bool   `json:"enabled"`
}

type templateToggleResponse struct {
	VMID    int  `json:"vmid"`
	Enabled bool `json:"enabled"`
}

// ServeTemplateToggle handles POST /api/v1/admin/templates/toggle.
func (h *AdminCatalog) ServeTemplateToggle(w http.ResponseWriter, r *http.Request) {
	var req templateToggleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	if req.VMID == 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return
	}

	// catalog.SetTemplateEnabled fetches the discovery set once, finds the
	// template, extracts its values for the first-approval insert, and
	// upserts the enabled state. No pre-fetch here - that would duplicate
	// the cluster round-trip.
	err = catalog.SetTemplateEnabled(r.Context(), h.store, client, clusterName, catalog.TemplateRef{VMID: req.VMID}, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("template vmid %d not found in cluster", req.VMID))
		return
	}

	if errors.Is(err, catalog.ErrTemplateUnreadable) {
		writeAdminError(w, http.StatusBadRequest, "template_unreadable", fmt.Sprintf("template vmid %d disk could not be read; it cannot be approved", req.VMID))
		return
	}

	if err != nil {
		writeAdminFailure(w, "admin toggle template failed", err)

		return
	}

	h.recordAdminAction(r, "admin.templates.toggle", "template", strconv.Itoa(req.VMID),
		fmt.Sprintf("template vmid %d cluster %s set enabled=%v", req.VMID, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, "vmid": req.VMID, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, templateToggleResponse{VMID: req.VMID, Enabled: req.Enabled})
}

// ServeTemplateDelete handles DELETE /api/v1/admin/templates/{cluster}/{vmid}
// removes an approval row - the UI offers Remove only on missing
// (orphaned) rows, but the API deletes any approval.
func (h *AdminCatalog) ServeTemplateDelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")

	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil || vmid <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "template vmid is required")

		return
	}

	err = catalog.DeleteTemplate(r.Context(), h.store, clusterName, vmid)
	if errors.Is(err, catalog.ErrTemplateNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("template vmid %d not found on cluster %q", vmid, clusterName))

		return
	}

	if err != nil {
		writeAdminFailure(w, "admin delete template failed", err)

		return
	}

	h.recordAdminAction(r, "admin.templates.delete", "template", strconv.Itoa(vmid),
		fmt.Sprintf("deleted template approval vmid %d on cluster %s", vmid, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "vmid": vmid}})
	w.WriteHeader(http.StatusNoContent)
}

// templateUpdateRequest is the body of PUT /api/v1/admin/templates/{cluster}/{vmid}
// (override_discovery flag). The cluster is taken from the path to match the
// delete handler's convention; the body carries the editable field values.
type templateUpdateRequest struct {
	Node             string `json:"node"`
	Name             string `json:"name"`
	CloudInitCapable bool   `json:"cloudInitCapable"`
	DiskStorage      string `json:"diskStorage"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	DiskBus          string `json:"diskBus"`
}

// ServeTemplateUpdate handles PUT /api/v1/admin/templates/{cluster}/{vmid}.
// Overrides the discovered template field values and pins the row against
// discovery-wins write-back. The create path still enforces the
// gabarit on clones, so an override above the gabarit simply means clones
// from this template are rejected at create time - the admin owns that.
func (h *AdminCatalog) ServeTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")

	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil || vmid <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "template vmid is required")

		return
	}

	var req templateUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)

		return
	}

	if req.Node == "" || req.DiskStorage == "" || req.DiskBus == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "node, diskStorage, and diskBus are required")

		return
	}

	if req.DiskSizeGB < 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "diskSizeGB must not be negative")

		return
	}

	values := store.TemplateValues{
		Node: req.Node, Name: req.Name, CloudInitCapable: req.CloudInitCapable,
		DiskStorage: req.DiskStorage, DiskSizeGB: req.DiskSizeGB, DiskBus: req.DiskBus,
	}
	if err := catalog.UpdateTemplate(r.Context(), h.store, clusterName, vmid, values); err != nil {
		if errors.Is(err, catalog.ErrTemplateNotFound) {
			writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("template vmid %d not found on cluster %q", vmid, clusterName))

			return
		}

		writeAdminFailure(w, "admin update template failed", err)

		return
	}

	h.recordAdminAction(r, "admin.templates.update", "template", strconv.Itoa(vmid),
		fmt.Sprintf("overrode template vmid %d on cluster %s: node=%s name=%q disk=%dGB@%s bus=%s cloudinit=%v",
			vmid, clusterName, req.Node, req.Name, req.DiskSizeGB, req.DiskStorage, req.DiskBus, req.CloudInitCapable),
		[]any{map[string]any{
			auditKeyCluster: clusterName, "vmid": vmid, auditKeyNode: req.Node, "name": req.Name,
			"diskStorage": req.DiskStorage, "diskSizeGB": req.DiskSizeGB, "diskBus": req.DiskBus,
			"cloudInitCapable": req.CloudInitCapable,
		}})
	writeAdminJSON(w, http.StatusOK, adminTemplateDTO{
		VMID: vmid, Node: req.Node, Name: req.Name, CloudInitCapable: req.CloudInitCapable,
		DiskStorage: req.DiskStorage, DiskSizeGB: req.DiskSizeGB, DiskBus: req.DiskBus,
		OverrideDiscovery: true,
	})
}

//  - helpers -
