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
	"regexp"
	"strings"
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
	auth             *Auth
	store            *store.Store
	clients          runtimeClusterRegistry
	inventories      *inventory.Registry
	log              *slog.Logger
	trustedProxyHops int
	// sshPublicKey is PVMSS's public key (authorized_keys form), shown so
	// the admin can install it on the nodes. Empty: no PVMSS_SSH_KEY_FILE.
	sshPublicKey string
	republisher  func(name string)
}

// SetRepublisher wires the background republication run after a cluster's
// settings are saved (nil in tests: nothing runs behind their back).
func (handler *AdminClusters) SetRepublisher(fn func(name string)) {
	handler.republisher = fn
}

func (handler *AdminClusters) republish(name string) {
	if handler.republisher != nil {
		handler.republisher(name)
	}
}

// SetSSHPublicKey sets the public key shown in the cluster form.
func (handler *AdminClusters) SetSSHPublicKey(key string) {
	handler.sshPublicKey = key
}

// hostKeyScanDTO is one node's scanned host key.
type hostKeyScanDTO struct {
	Node  string `json:"node"`
	Line  string `json:"line,omitempty"`
	Error string `json:"error,omitempty"`
}

// ServeSSHScan handles POST /api/v1/admin/clusters/{name}/ssh-scan: reads
// every node's SSH host key (trust on first use) so the admin can review
// the fingerprints and save them as the cluster's pinned host keys. Nothing
// is saved here.
func (handler *AdminClusters) ServeSSHScan(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	client, err := handler.clients.Client(name)
	if err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	publisher, ok := client.(cluster.SnippetPublisher)
	if !ok {
		writeAdminError(w, http.StatusConflict, "unsupported", "this cluster client cannot publish cloud-init documents")
		return
	}
	scans, err := publisher.ScanHostKeys(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusBadGateway, "scan_failed", err.Error())
		return
	}
	out := make([]hostKeyScanDTO, len(scans))
	for i, sc := range scans {
		out[i] = hostKeyScanDTO{Node: sc.Node, Line: sc.Line, Error: sc.Error}
	}
	writeAdminJSON(w, http.StatusOK, out)
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
	SnippetStorage        string  `json:"snippetStorage"`
	SSHUser               string  `json:"sshUser"`
	SSHPort               int     `json:"sshPort"`
	SSHKnownHosts         string  `json:"sshKnownHosts"`
	// SSHPublicKey is PVMSS's own public key (PVMSS_SSH_KEY_FILE), to
	// install on every node; empty when no key is configured.
	SSHPublicKey          string `json:"sshPublicKey"`
	CloudInitWriteEnabled bool   `json:"cloudInitWriteEnabled"`
	// PublishingStatus names the first missing prerequisite for cloud-init
	// publishing ("" when enabled), so Admin > Clusters can tell the admin
	// what to fix. The web client localizes the code.
	PublishingStatus string `json:"publishingStatus"`
}

type createClusterRequest struct {
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	TLSInsecureSkipVerify bool   `json:"tlsInsecureSkipVerify"`
	TokenID               string `json:"tokenId"`
	TokenSecret           string `json:"tokenSecret"`
	snippetSettings
}

type updateClusterRequest struct {
	URL                   string `json:"url"`
	TLSInsecureSkipVerify bool   `json:"tlsInsecureSkipVerify"`
	TokenID               string `json:"tokenId"`
	TokenSecret           string `json:"tokenSecret"`
	snippetSettings
}

// snippetSettings are the cloud-init publishing fields shared by the
// create and update requests.
type snippetSettings struct {
	SnippetStorage string `json:"snippetStorage"`
	SSHUser        string `json:"sshUser"`
	SSHPort        int    `json:"sshPort"`
	SSHKnownHosts  string `json:"sshKnownHosts"`
}

func (s snippetSettings) config() store.SnippetConfig {
	return store.SnippetConfig{Storage: s.SnippetStorage, SSHUser: s.SSHUser, SSHPort: s.SSHPort, KnownHosts: s.SSHKnownHosts}
}

// snippetStorageIDRE is the storage-id grammar - the same shape
// Proxmox itself accepts for a storage identifier.
var snippetStorageIDRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]*$`)

// sshUserRE is a conservative POSIX user name.
var sshUserRE = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// validateSnippetSettings checks the publishing settings. Returns the
// message for a 400 invalid_request, or "".
func validateSnippetSettings(s snippetSettings) string {
	switch {
	case s.SnippetStorage != "" && !snippetStorageIDRE.MatchString(s.SnippetStorage):
		return "snippetStorage is not a valid storage id"
	case s.SSHUser != "" && !sshUserRE.MatchString(s.SSHUser):
		return "sshUser is not a valid user name"
	case s.SSHUser == "root":
		return "sshUser must not be root: create the dedicated user with tools/pvmss-node-setup.sh"
	case s.SSHPort < 0 || s.SSHPort > 65535:
		return "sshPort must be between 1 and 65535"
	case s.SSHUser != "" && strings.TrimSpace(s.SSHKnownHosts) == "":
		return "sshKnownHosts is required: host keys are always verified (use Scan host keys)"
	}
	if err := cluster.ValidateKnownHosts(s.SSHKnownHosts); err != nil {
		return "sshKnownHosts: " + err.Error()
	}
	return ""
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
	if msg := validateSnippetSettings(request.snippetSettings); msg != "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}
	row := store.ClusterRow{Name: request.Name, URL: request.URL, TLSInsecureSkipVerify: request.TLSInsecureSkipVerify, TokenID: request.TokenID, TokenSecret: request.TokenSecret}
	if err := handler.store.CreateCluster(r.Context(), row); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	if err := handler.store.SetClusterSnippetConfig(r.Context(), row.Name, request.config()); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	row.SnippetStorage, row.SSHUser, row.SSHPort, row.SSHKnownHosts = request.SnippetStorage, request.SSHUser, request.SSHPort, request.SSHKnownHosts
	if err := handler.register(r.Context(), row); err != nil {
		handler.writeFailure(w, err)
		return
	}
	handler.awaitFirstRefresh(r.Context(), row.Name)
	handler.republish(row.Name)
	created, err := handler.store.GetCluster(r.Context(), row.Name)
	if err != nil {
		handler.writeFailure(w, err)
		return
	}
	handler.recordAdminAction(r, "admin.clusters.create", "cluster", created.Name,
		"created cluster "+created.Name,
		[]any{map[string]any{auditKeyName: created.Name, "url": created.URL, "tlsInsecureSkipVerify": created.TLSInsecureSkipVerify, "tokenId": created.TokenID, "oidcEnabled": created.OIDCEnabled, "snippetStorage": created.SnippetStorage, "sshUser": created.SSHUser, "sshPort": created.SSHPort}})
	writeAdminJSON(w, http.StatusCreated, handler.clusterDTO(created))
}

// ServeUpdate handles PUT /api/v1/admin/clusters/:name without accepting a name field.
func (handler *AdminClusters) ServeUpdate(w http.ResponseWriter, r *http.Request) {
	var request updateClusterRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid cluster request")
		return
	}
	if msg := validateSnippetSettings(request.snippetSettings); msg != "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msg)
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
	if err := handler.store.SetClusterSnippetConfig(r.Context(), name, request.config()); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}
	// Re-fetch the stored row so the registry factory receives the decrypted
	// token secret. The HTTP request omits TokenSecret on edit (the field is
	// only required on create), so the in-memory row above has it empty -
	// passing that to replace() would fail with "cluster credentials are
	// required" on the Proxmox factory.
	stored, err := handler.store.GetCluster(r.Context(), name)
	if err != nil {
		handler.writeFailure(w, err)
		return
	}
	if err := handler.replace(r.Context(), stored); err != nil {
		handler.writeFailure(w, err)
		return
	}
	handler.awaitFirstRefresh(r.Context(), name)
	handler.republish(name)
	updated, err := handler.store.GetCluster(r.Context(), name)
	if err != nil {
		handler.writeFailure(w, err)
		return
	}
	handler.recordAdminAction(r, "admin.clusters.update", "cluster", name,
		"updated cluster "+name,
		[]any{map[string]any{auditKeyName: name, "url": updated.URL, "tlsInsecureSkipVerify": updated.TLSInsecureSkipVerify, "tokenId": updated.TokenID, "oidcEnabled": updated.OIDCEnabled, "snippetStorage": updated.SnippetStorage, "sshUser": updated.SSHUser, "sshPort": updated.SSHPort}})
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
	handler.recordAdminAction(r, "admin.clusters.oidc", "cluster", name,
		fmt.Sprintf("set cluster %s OIDC enabled=%v", name, request.Enabled),
		[]any{map[string]any{auditKeyName: name, "oidcEnabled": request.Enabled}})
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
	handler.recordAdminAction(r, "admin.clusters.delete", "cluster", name,
		"removed cluster "+name,
		[]any{map[string]any{auditKeyName: name, "status": "removed"}})
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
			// row remains - the operator can retry or remove it via the API.
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
			// no client to roll back. The inventory entry is simply absent -
			// the next manual refresh or worker restart will not repopulate it
			// automatically. Surface the error so the operator knows to retry.
			return fmt.Errorf("rebuild inventory for %q: %w", row.Name, err)
		}
	}
	return nil
}

// awaitFirstRefresh waits for the freshly (re)built inventory entry's first
// refresh so the create/update response carries the cluster's real
// reachability, version, and node/VM counts instead of a transient
// "unreachable" derived from the just-wiped index. The call joins the
// worker's already-running initial refresh via singleflight - it does not
// issue a second Proxmox call. Errors are intentionally ignored: a failed
// refresh leaves the index empty and clusterDTO reports that accurately.
func (handler *AdminClusters) awaitFirstRefresh(ctx context.Context, name string) {
	if handler.inventories == nil {
		return
	}
	_, _ = handler.inventories.Refresh(ctx, name)
}

func (handler *AdminClusters) clusterDTO(row store.ClusterRow) adminClusterDTO {
	index := (*inventory.Index)(nil)
	fresh := false
	if handler.inventories != nil {
		index, _ = handler.inventories.Index(row.Name)
		fresh = handler.inventories.IsIndexFresh(row.Name)
	}

	version := row.ProxmoxVersion
	nodeCount, vmCount := 0, 0
	if index != nil {
		nodeCount, vmCount = len(index.Nodes), len(index.ByVMID)
		// Prefer the live inventory version - the background worker refreshes
		// it every interval, so it tracks cluster upgrades without a manual
		// Test. Fall back to the persisted DB value only when the index is
		// cold or has no version (cluster unreachable at last refresh).
		if index.ProxmoxVersion != "" {
			version = index.ProxmoxVersion
		}
	}

	lastTestStatus := row.LastTestStatus
	lastTestAt := formatTime(row.LastTestAt)
	lastTestMessage := row.LastTestMessage

	// Surface the current reachability in real time. If the last manual test
	// reported "ok" but the inventory index is now stale or missing, the
	// cluster is no longer reachable. Override the stale "ok" status so the
	// admin page does not claim the cluster is healthy (issue: cluster down
	// but admin page still shows "ok"). The message is left empty - the
	// frontend localizes a hint based on the "unreachable" status.
	if !fresh && lastTestStatus != nil && *lastTestStatus == "ok" {
		status := "unreachable"
		lastTestStatus = &status
		lastTestAt = nil
		lastTestMessage = nil
		version = ""
		nodeCount, vmCount = 0, 0
	}

	status := publishingStatus(row, handler.sshPublicKey)

	return adminClusterDTO{
		Name: row.Name, DisplayName: row.DisplayName, URL: row.URL, TLSInsecureSkipVerify: row.TLSInsecureSkipVerify, TokenID: row.TokenID,
		TokenSet: row.TokenSecret != "", OIDCEnabled: row.OIDCEnabled, RemovedAt: formatTime(row.RemovedAt),
		LastTestStatus: lastTestStatus, LastTestAt: lastTestAt, LastTestMessage: lastTestMessage,
		ProxmoxVersion: optionalValue(version), NodeCount: nodeCount, VMCount: vmCount,
		SnippetStorage: row.SnippetStorage, SSHUser: row.SSHUser, SSHPort: row.SSHPort, SSHKnownHosts: row.SSHKnownHosts,
		SSHPublicKey:          handler.sshPublicKey,
		CloudInitWriteEnabled: status == "",
		PublishingStatus:      status,
	}
}

// publishingStatus reports why cloud-init publishing is off for a cluster, or
// "" when every prerequisite is present: the global key (PVMSS_SSH_KEY_FILE),
// the per-cluster SSH user, pinned host keys, and the snippet storage. The
// first missing item wins; the web client localizes each code.
func publishingStatus(row store.ClusterRow, sshPublicKey string) string {
	switch {
	case sshPublicKey == "":
		return "no_ssh_key"
	case row.SSHUser == "":
		return "no_ssh_user"
	case strings.TrimSpace(row.SSHKnownHosts) == "":
		return "no_host_keys"
	case row.SnippetStorage == "":
		return "no_snippet_storage"
	default:
		return ""
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
	switch {
	case errors.Is(err, cluster.ErrTLSVerify):
		return "TLS certificate verification failed - enable \"Skip TLS certificate verification\" if the cluster uses a self-signed certificate"
	case errors.Is(err, cluster.ErrUnreachable):
		return "connection refused"
	default:
		return "cluster unreachable"
	}
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops to trust for
// client IP extraction used in audit log entries.
func (handler *AdminClusters) SetTrustedProxyHops(n int) {
	handler.trustedProxyHops = n
}

func (handler *AdminClusters) recordAdminAction(r *http.Request, action, targetType, targetID, summary string, changes []any) {
	actor, _ := handler.auth.Principal(r)
	if err := handler.store.RecordAdminAction(r.Context(), actor.Username, action, targetType, targetID, detailJSON(summary, changes), clientIP(r, handler.trustedProxyHops)); err != nil {
		handler.log.Error("failed to record admin action", "component", "httpapi", "action", action, "error", err)
	}
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
