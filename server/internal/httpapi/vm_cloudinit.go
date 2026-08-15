package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
	"time"
)

const maxCloudInitSnippetBody = 128 * 1024

// VMCloudInit serves the four per-VM cloud-init endpoints.
type VMCloudInit struct {
	projection *inventory.Projection
	auth       *Auth
	reader     cluster.CloudInitReader
	writer     cluster.Writer
	store      *store.Store
	refresher  vm.IndexRefresher
	policy     *policy.Policy
	log        *slog.Logger
}

// NewVMCloudInit creates the dedicated cloud-init handler.
func NewVMCloudInit(projection *inventory.Projection, authHandler *Auth, reader cluster.CloudInitReader, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger, services ...*policy.Policy) *VMCloudInit {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	if policyService == nil && st != nil {
		policyService = policy.New(st, projection, nil)
	}

	return &VMCloudInit{projection: projection, auth: authHandler, reader: reader, writer: writer, store: st, refresher: refresher, policy: policyService, log: log}
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

type cloudInitUpdateRequest struct {
	User         *string                  `json:"user"`
	Password     *string                  `json:"password"`
	SSHKeys      *[]string                `json:"sshKeys"`
	IPMode       *cluster.CloudInitIPMode `json:"ipMode"`
	IPAddress    *string                  `json:"ipAddress"`
	Gateway      *string                  `json:"gateway"`
	DNSServer    *string                  `json:"dnsServer"`
	SearchDomain *string                  `json:"searchDomain"`
	RebootNow    bool                     `json:"rebootNow"`
}

type cloudInitUpdateResponse struct {
	Status   string `json:"status"`
	Rebooted bool   `json:"rebooted"`
}

type cloudInitSnippetDTO struct {
	Content   *string `json:"content"`
	UpdatedAt *string `json:"updatedAt"`
	UpdatedBy *string `json:"updatedBy"`
}

type cloudInitSnippetRequest struct {
	Content *string `json:"content"`
}

type cloudInitSnippetResponse struct {
	Status string `json:"status"`
}

// ServeHTTP dispatches config and snippet routes by path suffix.
func (h *VMCloudInit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/cloudinit/snippet") {
		h.handleSnippet(w, r)
		return
	}

	h.handleConfig(w, r)
}

type cloudInitRouteHandler func(http.ResponseWriter, *http.Request, auth.Identity, string, int)

func (h *VMCloudInit) handleConfig(w http.ResponseWriter, r *http.Request) {
	h.serveRoute(w, r, h.getConfig, h.putConfig)
}

func (h *VMCloudInit) serveRoute(w http.ResponseWriter, r *http.Request, getHandler, putHandler cloudInitRouteHandler) {
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

	switch r.Method {
	case http.MethodGet:
		getHandler(w, r, identity, clusterName, vmid)
	case http.MethodPut:
		putHandler(w, r, identity, clusterName, vmid)
	default:
		w.Header().Set("Allow", "GET, PUT")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

func (h *VMCloudInit) getConfig(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return
	}

	config, err := vm.GetCloudInitConfig(r.Context(), index, actor, clusterName, vmid, h.reader)
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

func (h *VMCloudInit) putConfig(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	var request cloudInitUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_config", msgInvalidRequestBody)
		return
	}

	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return
	}

	rebooted, err := vm.SetCloudInitConfig(r.Context(), vm.CloudInitConfigDeps{
		Index: index, Actor: actor, ClusterName: clusterName, VMID: vmid,
		Reader: h.reader, Writer: h.writer, Audit: h.store, Refresher: h.refresher,
	}, cluster.CloudInitUpdate{
		User: request.User, Password: request.Password, SSHKeys: request.SSHKeys, IPMode: request.IPMode,
		IPAddress: request.IPAddress, Gateway: request.Gateway, DNSServer: request.DNSServer, SearchDomain: request.SearchDomain,
	}, request.RebootNow)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, cloudInitUpdateResponse{Status: "updated", Rebooted: rebooted})
}

func (h *VMCloudInit) handleSnippet(w http.ResponseWriter, r *http.Request) {
	h.serveRoute(w, r, h.getSnippet, h.putSnippet)
}

func (h *VMCloudInit) getSnippet(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return
	}

	snippet, found, err := vm.GetCloudInitSnippet(r.Context(), index, actor, clusterName, vmid, h.store)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}

	if !found {
		h.writeJSONStatus(w, http.StatusOK, cloudInitSnippetDTO{})
		return
	}

	content := snippet.Content
	updatedAt := snippet.UpdatedAt.Format(time.RFC3339Nano)
	updatedBy := snippet.UpdatedBy
	h.writeJSONStatus(w, http.StatusOK, cloudInitSnippetDTO{Content: &content, UpdatedAt: &updatedAt, UpdatedBy: &updatedBy})
}

func (h *VMCloudInit) putSnippet(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	var request cloudInitSnippetRequest
	if err := decodeJSONLimit(w, r, &request, maxCloudInitSnippetBody); err != nil || request.Content == nil {
		h.writeError(w, http.StatusBadRequest, "invalid_snippet", "content is required")
		return
	}

	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return
	}

	if err := vm.SetCloudInitSnippet(r.Context(), vm.CloudInitSnippetDeps{
		Index: index, Actor: actor, ClusterName: clusterName, VMID: vmid,
		Reader: h.reader, Writer: h.writer, Store: h.store, Service: h.policy,
	}, *request.Content); err != nil {
		h.writeDomainError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, cloudInitSnippetResponse{Status: "saved"})
}

func (h *VMCloudInit) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrCustomYAMLDisabled):
		h.writeError(w, http.StatusForbidden, "custom_yaml_disabled", "the administrator has disabled custom cloud-init snippets")
	case errors.Is(err, policy.ErrUnavailable):
		h.writeError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrInvalidCloudInitConfig):
		h.writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
	case errors.Is(err, cloudinit.ErrSnippetPrefix), errors.Is(err, cloudinit.ErrSnippetTooLarge), errors.Is(err, cloudinit.ErrSnippetInvalidUTF8):
		h.writeError(w, http.StatusBadRequest, "invalid_snippet", err.Error())
	case errors.Is(err, vm.ErrSnippetPushFailed):
		h.writeError(w, http.StatusBadGateway, "push_failed", "snippet saved, not yet applied to the VM")
	case errors.Is(err, cluster.ErrNotImplemented), errors.Is(err, cluster.ErrUnreachable), errors.Is(err, cluster.ErrNotFound):
		h.writeError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	default:
		h.log.Error("cloud-init request failed", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMCloudInit) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal cloud-init response", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write cloud-init response", "component", "httpapi", "error", err)
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
		h.log.Error("failed to write cloud-init error", "component", "httpapi", "error", err)
	}
}

func parseCloudInitPath(r *http.Request) (string, int, bool) {
	clusterName := r.PathValue("cluster")
	vmid, err := strconv.Atoi(r.PathValue("vmid"))

	return clusterName, vmid, clusterName != "" && err == nil && vmid > 0
}
