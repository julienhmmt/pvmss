package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"strconv"
)

// VMCloudInit serves the per-VM cloud-init read (SSH user for the Connect
// tab).
type VMCloudInit struct {
	projection *inventory.Projection
	resolver   vm.ClusterIndexResolver
	auth       *Auth
	reader     cluster.CloudInitReader
	clients    cluster.ClientProvider
	log        *slog.Logger
}

// VMCloudInitDeps groups the dependencies for constructing a VMCloudInit
// handler. Source and Clients are optional: when set (a multi-cluster
// deployment), every index load and cluster.CloudInitReader call resolves
// per-request from the request's own :cluster path value.
type VMCloudInitDeps struct {
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Auth       *Auth
	Reader     cluster.CloudInitReader
	Clients    cluster.ClientProvider
	Log        *slog.Logger
}

// NewVMCloudInit creates the dedicated cloud-init handler.
func NewVMCloudInit(deps VMCloudInitDeps) *VMCloudInit {
	resolver := vm.ClusterIndexResolver(registryResolver{registry: depsSourceRegistry(deps.Source)})

	return &VMCloudInit{projection: deps.Projection, resolver: resolver, auth: deps.Auth, reader: deps.Reader, clients: deps.Clients, log: deps.Log}
}

// index resolves the current Index for clusterName, writing the appropriate
// error response on failure.
func (h *VMCloudInit) index(w http.ResponseWriter, clusterName string) (*inventory.Index, bool) {
	return loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeError(w, status, code, message) })
}

// readerFor resolves the cluster.CloudInitReader for clusterName.
func (h *VMCloudInit) readerFor(w http.ResponseWriter, clusterName string) (cluster.CloudInitReader, bool) {
	reader, err := resolveCapability(h.clients, h.reader, clusterName, "CloudInitReader")
	if err != nil {
		h.writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return nil, false
	}

	return reader, true
}

type cloudInitConfigDTO struct {
	User         string                  `json:"user"`
	SSHKeys      []string                `json:"sshKeys"`
	IPMode       cluster.CloudInitIPMode `json:"ipMode"`
	IPAddress    string                  `json:"ipAddress,omitempty"`
	Gateway      string                  `json:"gateway,omitempty"`
	DNSServer    string                  `json:"dnsServer,omitempty"`
	SearchDomain string                  `json:"searchDomain,omitempty"`
}

// cloudInitDocumentDTO is the document a VM uses. TemplateID is "" when
// none, "__baseline__" for the standalone baseline; Legacy marks a per-VM
// document written before documents became admin-published.
// ServeHTTP serves the cloud-init read route.
func (h *VMCloudInit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleConfig(w, r)
}

func (h *VMCloudInit) handleConfig(w http.ResponseWriter, r *http.Request) {
	clusterName, vmid, ok := parseCloudInitPath(r)
	if !ok {
		h.writeError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

	h.getConfig(w, r, identity, clusterName, vmid)
}

func (h *VMCloudInit) getConfig(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	reader, ok := h.readerFor(w, clusterName)
	if !ok {
		return
	}

	config, err := vm.GetCloudInitConfig(r.Context(), index, actor, clusterName, vmid, reader)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}

	sshKeys := append([]string{}, config.SSHKeys...)
	h.writeJSONStatus(w, http.StatusOK, cloudInitConfigDTO{
		User: config.User, SSHKeys: sshKeys, IPMode: config.IPMode,
		IPAddress: config.IPAddress, Gateway: config.Gateway, DNSServer: config.DNSServer, SearchDomain: config.SearchDomain,
	})
}

func (h *VMCloudInit) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrNotApproved):
		h.writeError(w, http.StatusBadRequest, "not_approved", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case h.writeClusterFailure(w, err):
		// Already written: the cluster failed or refused the call.
	default:
		SetErrorMsg(w, "cloud-init read failed", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

// writeClusterFailure maps a cluster-side failure (unreachable, missing, or a
// Proxmox rejection such as a missing privilege) and reports true when it
// wrote the response.
func (h *VMCloudInit) writeClusterFailure(w http.ResponseWriter, err error) bool {
	if writeClusterRejection(w, err, h.log) {
		return true
	}

	if errors.Is(err, cluster.ErrNotImplemented) || errors.Is(err, cluster.ErrUnreachable) || errors.Is(err, cluster.ErrNotFound) {
		h.writeError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
		return true
	}

	return false
}

func (h *VMCloudInit) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		SetErrorMsg(w, "failed to marshal cloud-init response", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Warn("failed to write cloud-init response", "component", "httpapi", "error", err)
	}
}

func (h *VMCloudInit) writeError(w http.ResponseWriter, status int, code, message string) {
	body, err := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
	if err != nil {
		h.log.Error("failed to marshal cloud-init error", "component", "httpapi", "error", err)
		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Warn("failed to write cloud-init error", "component", "httpapi", "error", err)
	}
}

func parseCloudInitPath(r *http.Request) (string, int, bool) {
	clusterName := r.PathValue("cluster")
	vmid, err := strconv.Atoi(r.PathValue("vmid"))

	return clusterName, vmid, clusterName != "" && err == nil && vmid > 0
}
