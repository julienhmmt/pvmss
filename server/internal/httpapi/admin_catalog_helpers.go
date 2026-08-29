package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/store"
	"strings"
)

// catalogDeleteFunc deletes one catalog item by cluster and id.
type catalogDeleteFunc func(ctx context.Context, st *store.Store, cluster, id string) error

// catalogToggleFunc sets the enabled state of one catalog item.
type catalogToggleFunc func(ctx context.Context, st *store.Store, cluster, id string, enabled bool) error

// catalogToggleRequest is the common request body for toggle endpoints.
type catalogToggleRequest struct {
	Cluster string `json:"cluster"`
	Enabled bool   `json:"enabled"`
}

// catalogToggleResponse is the common response shape for toggle endpoints.
type catalogToggleResponse struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// serveCatalogDelete handles the common delete flow: validate the path id,
// resolve the cluster from the query string, invoke the catalog delete
// function, and map not-found / internal errors to the standard admin error
// responses. kind labels the item in user-facing messages (e.g. "profile");
// idLabel labels the id in the validation message (e.g. "profile id").
func (h *AdminCatalog) serveCatalogDelete(w http.ResponseWriter, r *http.Request, kind, idLabel string, del catalogDeleteFunc, notFound error) {
	id := r.PathValue("id")

	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", idLabel+" id is required")

		return
	}

	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)

		return
	}

	err := del(r.Context(), h.store, clusterName, id)
	if errors.Is(err, notFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", kind+" \""+id+"\" not found")

		return
	}

	if err != nil {
		h.log.Error("admin delete "+kind+" failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	h.recordAdminAction(r, catalogActionSlug(kind)+".delete", catalogTargetType(kind), id,
		fmt.Sprintf("deleted %s %s on cluster %s", kind, id, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "id": id}})
	writeAdminJSON(w, http.StatusOK, statusResponse{Status: statusDeleted})
}

// serveCatalogToggle handles the common toggle flow: validate the path id,
// decode the JSON body, resolve the cluster from the body, invoke the catalog
// toggle function, and map not-found / internal errors to the standard admin
// error responses. kind and idLabel are as in serveCatalogDelete.
func (h *AdminCatalog) serveCatalogToggle(w http.ResponseWriter, r *http.Request, kind, idLabel string, toggle catalogToggleFunc, notFound error) {
	id := r.PathValue("id")

	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", idLabel+" id is required")

		return
	}

	var req catalogToggleRequest
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

	err := toggle(r.Context(), h.store, clusterName, id, req.Enabled)
	if errors.Is(err, notFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", kind+" \""+id+"\" not found")

		return
	}

	if err != nil {
		h.log.Error("admin toggle "+kind+" failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	h.recordAdminAction(r, catalogActionSlug(kind)+".toggle", catalogTargetType(kind), id,
		fmt.Sprintf("toggled %s %s on cluster %s to enabled=%v", kind, id, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, "id": id, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, catalogToggleResponse{ID: id, Enabled: req.Enabled})
}

func catalogActionSlug(kind string) string {
	return "admin." + strings.ReplaceAll(strings.ReplaceAll(kind, "-", ""), " ", "") + "s"
}

func catalogTargetType(kind string) string {
	return strings.ReplaceAll(strings.ReplaceAll(kind, "-", "_"), " ", "_")
}
