//nolint:wsl_v5 // profile handlers keep cluster selection and catalog mapping adjacent
package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/catalog"
)

// --- Profile DTOs ---

type adminProfileDTO struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
	Enabled  bool   `json:"enabled"`
}

type profileCreateRequest struct {
	Cluster  string `json:"cluster"`
	Label    string `json:"label"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
}

type profileUpdateRequest struct {
	Cluster  string `json:"cluster"`
	Label    string `json:"label"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
}

type statusResponse struct {
	Status string `json:"status"`
}

// statusDeleted is the status string returned by delete endpoints.
const statusDeleted = "deleted"

// ServeProfiles handles GET /api/v1/admin/profiles — lists all profiles
// including disabled ones (unlike T06's catalog.Profiles which filters by
// enabled = 1).
func (h *AdminCatalog) ServeProfiles(w http.ResponseWriter, r *http.Request) {
	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	profiles, err := catalog.ListAdminProfiles(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("admin list profiles failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminProfileDTO, len(profiles))
	for i, p := range profiles {
		dto[i] = adminProfileDTO{
			ID: p.ID, Label: p.Label, CPUCores: p.CPUCores,
			MemoryMB: p.MemoryMB, DiskGB: p.DiskGB, Bus: p.Bus, Enabled: p.Enabled,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeProfileCreate handles POST /api/v1/admin/profiles.
func (h *AdminCatalog) ServeProfileCreate(w http.ResponseWriter, r *http.Request) {
	var req profileCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	profile, err := catalog.CreateProfile(r.Context(), h.store, clusterName, catalog.ProfileSpec{Label: req.Label, CPUCores: req.CPUCores, MemoryMB: req.MemoryMB, DiskGB: req.DiskGB, Bus: req.Bus})
	if errors.Is(err, catalog.ErrDuplicateProfile) {
		writeAdminError(w, http.StatusConflict, "duplicate_profile", "a profile with this label already exists")
		return
	}

	if errors.Is(err, catalog.ErrInvalidProfile) {
		writeAdminError(w, http.StatusBadRequest, "invalid_profile", err.Error())
		return
	}

	if err != nil {
		h.log.Error("admin create profile failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusCreated, adminProfileDTO{
		ID: profile.ID, Label: profile.Label, CPUCores: profile.CPUCores,
		MemoryMB: profile.MemoryMB, DiskGB: profile.DiskGB, Bus: profile.Bus, Enabled: profile.Enabled,
	})
}

// ServeProfileUpdate handles PUT /api/v1/admin/profiles/{id}.
func (h *AdminCatalog) ServeProfileUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "profile id is required")
		return
	}

	var req profileUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	profile, err := catalog.UpdateProfile(r.Context(), h.store, clusterName, id, catalog.ProfileSpec{Label: req.Label, CPUCores: req.CPUCores, MemoryMB: req.MemoryMB, DiskGB: req.DiskGB, Bus: req.Bus})
	if errors.Is(err, catalog.ErrProfileNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "profile \""+id+"\" not found")
		return
	}

	if errors.Is(err, catalog.ErrInvalidProfile) {
		writeAdminError(w, http.StatusBadRequest, "invalid_profile", err.Error())
		return
	}

	if err != nil {
		h.log.Error("admin update profile failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusOK, adminProfileDTO{
		ID: profile.ID, Label: profile.Label, CPUCores: profile.CPUCores,
		MemoryMB: profile.MemoryMB, DiskGB: profile.DiskGB, Bus: profile.Bus, Enabled: profile.Enabled,
	})
}

// ServeProfileDelete handles DELETE /api/v1/admin/profiles/{id}. The cluster
// is read from the query string (?cluster=default), not the JSON body —
// DELETE-with-body is awkward and the frontend uses the query param form.
func (h *AdminCatalog) ServeProfileDelete(w http.ResponseWriter, r *http.Request) {
	h.serveCatalogDelete(w, r, "profile", "profile", catalog.DeleteProfile, catalog.ErrProfileNotFound)
}

// ServeProfileToggle handles POST /api/v1/admin/profiles/{id}/toggle.
func (h *AdminCatalog) ServeProfileToggle(w http.ResponseWriter, r *http.Request) {
	h.serveCatalogToggle(w, r, "profile", "profile", catalog.SetProfileEnabled, catalog.ErrProfileNotFound)
}
