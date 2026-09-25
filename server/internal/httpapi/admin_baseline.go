//nolint:wsl_v5 // admin endpoint handlers keep validation and response mapping adjacent
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// AdminBaseline serves the admin-only read-only view of the generated
// cloud-init baseline: the document merged into every published template
// and published on its own for image VMs created without a template, with
// its per-node publication state.
type AdminBaseline struct {
	auth    *Auth
	clients cluster.ClientProvider
	store   *store.Store
	log     *slog.Logger
}

// NewAdminBaseline creates the admin baseline handler.
func NewAdminBaseline(authHandler *Auth, clients cluster.ClientProvider, st *store.Store, log *slog.Logger) *AdminBaseline {
	return &AdminBaseline{auth: authHandler, clients: clients, store: st, log: log}
}

// adminBaselineDTO is the API response for the admin baseline view.
type adminBaselineDTO struct {
	// Generated is the baseline document PVMSS publishes, verbatim (the
	// same cloudinit.BuildVendorData output merged into every template).
	Generated string `json:"generated"`
	// Publication is the latest publication of the standalone baseline on
	// the cluster (image VMs without a template), nil when never published.
	Publication *adminPublicationDTO `json:"publication"`
}

// ServeBaseline handles GET /api/v1/admin/baseline?cluster=<name>: returns
// the generated baseline document and the cluster's override state.
func (h *AdminBaseline) ServeBaseline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")

		return
	}

	clusterName := r.URL.Query().Get("cluster")
	if clusterName == "" {
		// Single-cluster deployments without a registry: use the default.
		if h.clients != nil {
			names := h.clients.List()
			if len(names) > 0 {
				clusterName = names[0]
			}
		}
		if clusterName == "" {
			clusterName = "default"
		}
	}

	// The generated baseline is the same document the create path builds
	// with no override and no user document.
	generated, err := cloudinit.BuildVendorData(cloudinit.BaselineInputs{})
	if err != nil {
		// The generated baseline is built from constants - a failure here
		// is a programmer error, not an operator condition.
		h.writeError(w, http.StatusInternalServerError, "baseline_build_failed", "could not build the generated baseline")

		return
	}

	dto := adminBaselineDTO{Generated: generated}

	if h.store != nil {
		publication, found, err := h.store.GetCloudInitPublication(r.Context(), clusterName, store.BaselineTemplateID)
		if err != nil {
			h.log.Error("read baseline publication failed", "component", "httpapi", "cluster", clusterName, "error", err)
		} else if found {
			pub := publicationDTO(publication)
			dto.Publication = &pub
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto); err != nil {
		h.log.Error("encode baseline response failed", "component", "httpapi", "error", err)
	}
}

func (h *AdminBaseline) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message}); err != nil {
		h.log.Error("encode error response failed", "component", "httpapi", "error", errors.New(code))
	}
}
