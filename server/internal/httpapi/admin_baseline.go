//nolint:wsl_v5 // admin endpoint handlers keep validation and response mapping adjacent
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// AdminBaseline serves the admin-only read-only view of the generated
// cloud-init baseline (cloud-image-console issue 07). The baseline is the
// document the create path would deliver to a new image-mode VM on this
// cluster; the page also reports whether a cluster-wide pvmss-baseline.yml
// override is present.
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
	// Generated is the baseline document the create path would deliver,
	// verbatim. Read from the same source (cloudinit.BuildVendorData with
	// no override and no user document), not a copy.
	Generated string `json:"generated"`
	// OverridePresent is true when a cluster-wide pvmss-baseline.yml
	// exists in the cluster's snippet storage.
	OverridePresent bool `json:"overridePresent"`
	// OverrideFilename is the filename the create path looks for.
	OverrideFilename string `json:"overrideFilename"`
	// OverrideContent is the override document's content when present,
	// empty otherwise. Read live so the admin sees the current file.
	OverrideContent string `json:"overrideContent,omitempty"`
	// OverrideError carries a read failure reason when the override
	// exists but could not be read (best-effort: the page still shows
	// the generated baseline).
	OverrideError string `json:"overrideError,omitempty"`
}

const baselineOverrideFilename = "pvmss-baseline.yml"

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
	// with no override and no user document (issue 07: "the document shown
	// is the same one the create path would deliver — the page reads it
	// from the same source, not a copy").
	generated, err := cloudinit.BuildVendorData(cloudinit.BaselineInputs{})
	if err != nil {
		// The generated baseline is built from constants — a failure here
		// is a programmer error, not an operator condition.
		h.writeError(w, http.StatusInternalServerError, "baseline_build_failed", "could not build the generated baseline")

		return
	}

	dto := adminBaselineDTO{
		Generated:        generated,
		OverrideFilename: baselineOverrideFilename,
	}

	// Read the override state live from the cluster's snippet storage.
	// Best-effort: a read failure does not hide the generated baseline.
	dto.OverridePresent, dto.OverrideContent, dto.OverrideError = h.readOverrideState(r.Context(), clusterName)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto); err != nil {
		h.log.Error("encode baseline response failed", "component", "httpapi", "error", err)
	}
}

// readOverrideState reads the cluster-wide pvmss-baseline.yml override
// from the cluster's snippet storage. Returns (present, content, error).
// Best-effort: any failure returns the error with present=false and an
// empty content, so the page still shows the generated baseline.
func (h *AdminBaseline) readOverrideState(ctx context.Context, clusterName string) (bool, string, string) {
	if h.clients == nil {
		return false, "", ""
	}

	client, err := h.clients.Client(clusterName)
	if err != nil {
		return false, "", err.Error()
	}

	// Snapshot gives the node list; FindSnippetStorage needs a node to
	// locate the snippet-capable storage. The first node is sufficient —
	// the override is cluster-wide.
	snapshot, snapshotErr := client.Snapshot(ctx)
	if snapshotErr != nil {
		return false, "", snapshotErr.Error()
	}

	if len(snapshot.Nodes) == 0 {
		return false, "", ""
	}

	node := snapshot.Nodes[0].Name

	// FindSnippetStorage is on CloudInitReader; HasSnippet and ReadSnippet
	// are on Writer. The cluster client implements both.
	reader, readerOk := client.(cluster.CloudInitReader)
	writer, writerOk := client.(cluster.Writer)
	if !readerOk || !writerOk {
		return false, "", "cluster client does not support snippet reads"
	}

	storage, storageErr := reader.FindSnippetStorage(ctx, node)
	if storageErr != nil {
		return false, "", storageErr.Error()
	}

	if storage == "" {
		return false, "", ""
	}

	present, presentErr := writer.HasSnippet(ctx, node, storage, baselineOverrideFilename)
	if presentErr != nil {
		return false, "", presentErr.Error()
	}

	if !present {
		return false, "", ""
	}

	content, readErr := writer.ReadSnippet(ctx, node, storage, baselineOverrideFilename)
	if readErr != nil {
		return true, "", readErr.Error()
	}

	return true, content, ""
}

func (h *AdminBaseline) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message}); err != nil {
		h.log.Error("encode error response failed", "component", "httpapi", "error", errors.New(code))
	}
}
