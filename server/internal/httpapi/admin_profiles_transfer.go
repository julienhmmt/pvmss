//nolint:wsl_v5 // profile handlers keep cluster selection and catalog mapping adjacent
package httpapi

import (
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
)

const (
	// maxProfileImportBytes bounds an import body: 500 profiles with full
	// descriptions fit well under it.
	maxProfileImportBytes = 1 << 20
	maxProfileImportItems = 500
	maxProfileReorderIDs  = 1000
)

type profileReorderRequest struct {
	Cluster string   `json:"cluster"`
	IDs     []string `json:"ids"`
}

type profileImportItemDTO struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label"`
	Sockets     int    `json:"sockets"`
	CPUCores    int    `json:"cpuCores"`
	MemoryMB    int    `json:"memoryMB"`
	DiskGB      int    `json:"diskGB"`
	Bus         string `json:"bus"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Description string `json:"description"`
}

type profileImportRequest struct {
	Clusters  []string               `json:"clusters"`
	Overwrite bool                   `json:"overwrite"`
	Profiles  []profileImportItemDTO `json:"profiles"`
}

type profileImportOutcomeDTO struct {
	Cluster string `json:"cluster"`
	ID      string `json:"id"`
	Label   string `json:"label"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

// ServeProfileReorder handles PUT /api/v1/admin/profiles/order - stores the
// display order used by both the admin list and the user-side picker.
func (h *AdminCatalog) ServeProfileReorder(w http.ResponseWriter, r *http.Request) {
	var req profileReorderRequest
	if err := decodeJSONLimit(w, r, &req, 64<<10); err != nil || len(req.IDs) > maxProfileReorderIDs {
		writeAdminError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		status, code, message := clusterParamError(clusterErr)
		writeAdminError(w, status, code, message)
		return
	}

	if err := catalog.ReorderProfiles(r.Context(), h.store, clusterName, req.IDs); err != nil {
		SetErrorMsg(w, "admin reorder profiles failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.profiles.reorder", "profile", clusterName,
		"reordered profiles on cluster "+clusterName,
		[]any{map[string]any{auditKeyCluster: clusterName, "ids": req.IDs}})
	writeAdminJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

// ServeProfileImport handles POST /api/v1/admin/profiles/import - writes the
// given profiles to each target cluster and reports a per-row outcome.
func (h *AdminCatalog) ServeProfileImport(w http.ResponseWriter, r *http.Request) {
	var req profileImportRequest
	if err := decodeJSONLimit(w, r, &req, maxProfileImportBytes); err != nil ||
		len(req.Profiles) == 0 || len(req.Profiles) > maxProfileImportItems || len(req.Clusters) == 0 {
		writeAdminError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	clusters := make([]string, 0, len(req.Clusters))
	for _, c := range req.Clusters {
		name, err := ResolveClusterValue(c, h.clusters)
		if err != nil {
			status, code, message := clusterParamError(err)
			writeAdminError(w, status, code, message)
			return
		}
		clusters = append(clusters, name)
	}

	items := make([]catalog.ImportItem, len(req.Profiles))
	for i, p := range req.Profiles {
		items[i] = catalog.ImportItem{
			Spec: catalog.ProfileSpec{
				Label: p.Label, Sockets: p.Sockets, CPUCores: p.CPUCores, MemoryMB: p.MemoryMB,
				DiskGB: p.DiskGB, Bus: p.Bus, Description: p.Description,
			},
			Enabled: p.Enabled == nil || *p.Enabled,
		}
	}

	outcomes, err := catalog.ImportProfiles(r.Context(), h.store, clusters, items, req.Overwrite)

	dto := make([]profileImportOutcomeDTO, len(outcomes))
	counts := map[string]int{}
	for i, o := range outcomes {
		dto[i] = profileImportOutcomeDTO{Cluster: o.Cluster, ID: o.ID, Label: o.Label, Result: o.Result, Error: o.Error}
		counts[o.Result]++
	}

	// A mid-import failure still returns the outcomes already written: audit
	// them so a partial import never lands silently.
	if len(outcomes) > 0 {
		h.recordAdminAction(r, "admin.profiles.import", "profile", "",
			fmt.Sprintf("imported profiles to %d cluster(s): %d created, %d updated, %d skipped, %d invalid",
				len(clusters), counts[catalog.ImportCreated], counts[catalog.ImportUpdated], counts[catalog.ImportSkipped], counts[catalog.ImportInvalid]),
			[]any{map[string]any{"clusters": clusters, "overwrite": req.Overwrite, "results": counts}})
	}
	if err != nil {
		SetErrorMsg(w, "admin import profiles failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	writeAdminJSON(w, http.StatusOK, dto)
}
