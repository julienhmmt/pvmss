//nolint:wsl_v5 // admin endpoint handlers keep validation and response mapping adjacent
package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"time"
)

// AdminClusters serves runtime cluster administration and connection tests.
type runtimeClusterRegistry interface {
	cluster.ClientProvider
	Add(context.Context, store.ClusterRow) error
	Update(context.Context, store.ClusterRow) error
	Remove(string)
}

// AdminClusters exposes runtime cluster administration endpoints.
type AdminClusters struct {
	auth        *Auth
	store       *store.Store
	clients     runtimeClusterRegistry
	inventories *inventory.Registry
	log         *slog.Logger
}

// NewAdminClusters creates the admin cluster handler.
func NewAdminClusters(authHandler *Auth, st *store.Store, clients runtimeClusterRegistry, inventories *inventory.Registry, log *slog.Logger) *AdminClusters {
	return &AdminClusters{auth: authHandler, store: st, clients: clients, inventories: inventories, log: log}
}

type adminClusterDTO struct {
	Name                  string  `json:"name"`
	DisplayName           string  `json:"displayName"`
	URL                   string  `json:"url"`
	TLSInsecureSkipVerify bool    `json:"tlsInsecureSkipVerify"`
	TokenID               string  `json:"tokenId"`
	TokenSet              bool    `json:"tokenSet"`
	OIDCEnabled           bool    `json:"oidcEnabled"`
	RemovedAt             *string `json:"removedAt"`
	LastTestStatus        *string `json:"lastTestStatus"`
	LastTestAt            *string `json:"lastTestAt"`
	LastTestMessage       *string `json:"lastTestMessage"`
	ProxmoxVersion        *string `json:"proxmoxVersion"`
	NodeCount             int     `json:"nodeCount"`
	VMCount               int     `json:"vmCount"`
}

type createClusterRequest struct {
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	TLSInsecureSkipVerify bool   `json:"tlsInsecureSkipVerify"`
	TokenID               string `json:"tokenId"`
	TokenSecret           string `json:"tokenSecret"`
}

type updateClusterRequest struct {
	URL                   string `json:"url"`
	TLSInsecureSkipVerify bool   `json:"tlsInsecureSkipVerify"`
	TokenID               string `json:"tokenId"`
	TokenSecret           string `json:"tokenSecret"`
}

type testClusterResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
	ProxmoxVersion string `json:"proxmoxVersion,omitempty"`
	NodeCount      int    `json:"nodeCount,omitempty"`
	VMCount        int    `json:"vmCount,omitempty"`
	TestedAt       string `json:"testedAt"`
}

type oidcClusterRequest struct {
	Enabled bool `json:"enabled"`
}

// ServeList handles GET /api/v1/admin/clusters.
func (handler *AdminClusters) ServeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	rows, err := handler.store.ListClusters(r.Context())
	if err != nil {
		handler.writeFailure(w, err)
		return
	}
	result := make([]adminClusterDTO, 0, len(rows))
	for _, row := range rows {
		result = append(result, handler.clusterDTO(row))
	}
	writeAdminJSON(w, http.StatusOK, result)
}

// ServeCreate handles POST /api/v1/admin/clusters.
func (handler *AdminClusters) ServeCreate(w http.ResponseWriter, r *http.Request) {
	var request createClusterRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid cluster request")
		return
	}
	row := store.ClusterRow{Name: request.Name, URL: request.URL, TLSInsecureSkipVerify: request.TLSInsecureSkipVerify, TokenID: request.TokenID, TokenSecret: request.TokenSecret}
	if err := handler.store.CreateCluster(r.Context(), row); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	if err := handler.register(r.Context(), row); err != nil {
		handler.writeFailure(w, err)
		return
	}
	created, err := handler.store.GetCluster(r.Context(), row.Name)
	if err != nil {
		handler.writeFailure(w, err)
		return
	}
	writeAdminJSON(w, http.StatusCreated, handler.clusterDTO(created))
}

// ServeUpdate handles PUT /api/v1/admin/clusters/:name without accepting a name field.
func (handler *AdminClusters) ServeUpdate(w http.ResponseWriter, r *http.Request) {
	var request updateClusterRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid cluster request")
		return
	}
	name := r.PathValue("name")
	row := store.ClusterRow{Name: name, URL: request.URL, TLSInsecureSkipVerify: request.TLSInsecureSkipVerify, TokenID: request.TokenID, TokenSecret: request.TokenSecret}
	if err := handler.store.UpdateCluster(r.Context(), row); err != nil {
		if errors.Is(err, store.ErrInvalidClusterName) {
			writeAdminError(w, http.StatusNotFound, "not_found", "cluster not found")
			return
		}
		handler.writeStoreFailure(w, err)
		return
	}
	if err := handler.replace(r.Context(), row); err != nil {
		handler.writeFailure(w, err)
		return
	}
	updated, err := handler.store.GetCluster(r.Context(), name)
	if err != nil {
		handler.writeFailure(w, err)
		return
	}
	writeAdminJSON(w, http.StatusOK, handler.clusterDTO(updated))
}

// ServeTest handles POST /api/v1/admin/clusters/:name/test.
func (handler *AdminClusters) ServeTest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := handler.store.GetCluster(r.Context(), name); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	client, err := handler.clients.Client(name)
	if err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	testedAt := time.Now().UTC()
	snapshot, err := client.Snapshot(r.Context())
	if err != nil {
		status := "unreachable"
		if !errors.Is(err, cluster.ErrUnreachable) {
			status = "error"
		}
		message := shortClusterError(err)
		if saveErr := handler.store.SetClusterTestResult(r.Context(), name, status, "", message, testedAt); saveErr != nil {
			handler.writeFailure(w, saveErr)
			return
		}
		writeAdminJSON(w, http.StatusOK, testClusterResponse{Status: status, Message: message, TestedAt: testedAt.Format(time.RFC3339Nano)})
		return
	}
	if handler.inventories != nil {
		if err := handler.inventories.StoreSnapshot(name, snapshot); err != nil {
			handler.log.Warn("publish cluster test snapshot failed", "component", "httpapi", "cluster", name, "error", err)
		}
	}
	if err := handler.store.SetClusterTestResult(r.Context(), name, "ok", snapshot.ProxmoxVersion, "", testedAt); err != nil {
		handler.writeFailure(w, err)
		return
	}
	if displayName, err := client.DisplayName(r.Context()); err != nil {
		handler.log.Warn("cluster display name discovery failed", "component", "httpapi", "cluster", name, "error", err)
	} else if displayName != "" {
		if err := handler.store.SetClusterDisplayName(r.Context(), name, displayName); err != nil {
			handler.log.Warn("cluster display name persist failed", "component", "httpapi", "cluster", name, "error", err)
		}
	}
	writeAdminJSON(w, http.StatusOK, testClusterResponse{Status: "ok", ProxmoxVersion: snapshot.ProxmoxVersion, NodeCount: len(snapshot.Nodes), VMCount: len(snapshot.VMs), TestedAt: testedAt.Format(time.RFC3339Nano)})
}

// ServeOIDC handles POST /api/v1/admin/clusters/:name/oidc.
func (handler *AdminClusters) ServeOIDC(w http.ResponseWriter, r *http.Request) {
	var request oidcClusterRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid OIDC request")
		return
	}
	name := r.PathValue("name")
	if err := handler.store.SetClusterOIDC(r.Context(), name, request.Enabled); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	writeAdminJSON(w, http.StatusOK, struct {
		Name        string `json:"name"`
		OIDCEnabled bool   `json:"oidcEnabled"`
	}{Name: name, OIDCEnabled: request.Enabled})
}

// ServeDelete handles DELETE /api/v1/admin/clusters/:name as a soft delete.
func (handler *AdminClusters) ServeDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := handler.store.SoftDeleteCluster(r.Context(), name); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	handler.clients.Remove(name)
	if handler.inventories != nil {
		handler.inventories.Remove(name)
	}
	writeAdminJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "removed"})
}

func (handler *AdminClusters) register(ctx context.Context, row store.ClusterRow) error {
	if err := handler.clients.Add(ctx, row); err != nil {
		return err
	}
	if handler.inventories != nil {
		if err := handler.inventories.Add(row.Name); err != nil {
			// Compensate: the store row is persisted and the client is active,
			// but inventory setup failed. Roll back the client registration so
			// the cluster is not half-wired (no projection/worker). The store
			// row remains — the operator can retry or remove it via the API.
			handler.clients.Remove(row.Name)
			return fmt.Errorf("register inventory for %q: %w", row.Name, err)
		}
	}
	return nil
}

func (handler *AdminClusters) replace(ctx context.Context, row store.ClusterRow) error {
	if err := handler.clients.Update(ctx, row); err != nil {
		return err
	}
	if handler.inventories != nil {
		handler.inventories.Remove(row.Name)
		if err := handler.inventories.Add(row.Name); err != nil {
			// Compensate: the client was already updated in place, so there is
			// no client to roll back. The inventory entry is simply absent —
			// the next manual refresh or worker restart will not repopulate it
			// automatically. Surface the error so the operator knows to retry.
			return fmt.Errorf("rebuild inventory for %q: %w", row.Name, err)
		}
	}
	return nil
}

func (handler *AdminClusters) clusterDTO(row store.ClusterRow) adminClusterDTO {
	index := (*inventory.Index)(nil)
	if handler.inventories != nil {
		index, _ = handler.inventories.Index(row.Name)
	}
	version := row.ProxmoxVersion
	nodeCount, vmCount := 0, 0
	if index != nil {
		nodeCount, vmCount = len(index.Nodes), len(index.ByVMID)
		if version == "" {
			version = index.ProxmoxVersion
		}
	}
	return adminClusterDTO{
		Name: row.Name, DisplayName: row.DisplayName, URL: row.URL, TLSInsecureSkipVerify: row.TLSInsecureSkipVerify, TokenID: row.TokenID,
		TokenSet: row.TokenSecret != "", OIDCEnabled: row.OIDCEnabled, RemovedAt: formatTime(row.RemovedAt),
		LastTestStatus: row.LastTestStatus, LastTestAt: formatTime(row.LastTestAt), LastTestMessage: row.LastTestMessage,
		ProxmoxVersion: optionalValue(version), NodeCount: nodeCount, VMCount: vmCount,
	}
}

func (handler *AdminClusters) writeStoreFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidClusterName):
		writeAdminError(w, http.StatusBadRequest, "invalid_cluster_name", "name must match [a-z0-9-]+")
	case errors.Is(err, store.ErrDuplicateCluster), errors.Is(err, inventory.ErrDuplicateCluster):
		writeAdminError(w, http.StatusConflict, "duplicate_cluster", err.Error())
	case errors.Is(err, store.ErrLastActiveCluster):
		writeAdminError(w, http.StatusConflict, "last_cluster", "cannot remove the only active cluster")
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, cluster.ErrClusterNotFound), errors.Is(err, inventory.ErrClusterNotFound):
		writeAdminError(w, http.StatusNotFound, "not_found", "cluster not found")
	default:
		handler.writeFailure(w, err)
	}
}

func (handler *AdminClusters) writeFailure(w http.ResponseWriter, err error) {
	handler.log.Error("admin cluster operation failed", "component", "httpapi", "error", err)
	writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func shortClusterError(err error) string {
	if errors.Is(err, cluster.ErrUnreachable) {
		return "connection refused"
	}
	return "cluster unreachable"
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	result := value.UTC().Format(time.RFC3339Nano)
	return &result
}

func optionalValue(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
