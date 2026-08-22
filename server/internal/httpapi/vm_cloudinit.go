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
	resolver   vm.ClusterIndexResolver
	auth       *Auth
	reader     cluster.CloudInitReader
	writer     cluster.Writer
	clients    cluster.ClientProvider
	store      *store.Store
	refresher  vm.IndexRefresher
	policy     *policy.Policy
	log        *slog.Logger
}

// VMCloudInitDeps groups the shared dependencies for constructing a VMCloudInit
// handler. It collapses the seven positional parameters NewVMCloudInit used to
// take (SonarQube go:S107). Source and Clients are optional: when set (a
// multi-cluster deployment), every index load and cluster.Reader/Writer call
// below resolves per-request from the request's own :cluster path value
// instead of the single bound Projection/Reader/Writer.
type VMCloudInitDeps struct {
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Auth       *Auth
	Reader     cluster.CloudInitReader
	Writer     cluster.Writer
	Clients    cluster.ClientProvider
	Store      *store.Store
	Refresher  vm.IndexRefresher
	Log        *slog.Logger
}

// NewVMCloudInit creates the dedicated cloud-init handler.
func NewVMCloudInit(deps VMCloudInitDeps, services ...*policy.Policy) *VMCloudInit {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	if policyService == nil && deps.Store != nil {
		policyService = policy.New(deps.Store, deps.Projection, nil)
	}

	resolver := vm.ClusterIndexResolver(singleClusterResolver{projection: deps.Projection})
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		resolver = registryResolver{registry: registry}
	}

	return &VMCloudInit{projection: deps.Projection, resolver: resolver, auth: deps.Auth, reader: deps.Reader, writer: deps.Writer, clients: deps.Clients, store: deps.Store, refresher: deps.Refresher, policy: policyService, log: deps.Log}
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

// writerFor resolves the cluster.Writer for clusterName.
func (h *VMCloudInit) writerFor(w http.ResponseWriter, clusterName string) (cluster.Writer, bool) {
	writer, err := resolveCapability(h.clients, h.writer, clusterName, "Writer")
	if err != nil {
		h.writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return nil, false
	}

	return writer, true
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

type cloudInitSSHKeyRequest struct {
	User string `json:"user"`
	Key  string `json:"key"`
}

type cloudInitSSHKeyResponse struct {
	Status string `json:"status"`
}

// ServeHTTP dispatches config, snippet, and ssh-key routes by path suffix.
func (h *VMCloudInit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/cloudinit/snippet"):
		h.handleSnippet(w, r)
	case strings.HasSuffix(r.URL.Path, "/cloudinit/ssh-keys"):
		h.handleSSHKey(w, r)
	default:
		h.handleConfig(w, r)
	}
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

func (h *VMCloudInit) putConfig(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	var request cloudInitUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_config", msgInvalidRequestBody)
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	reader, ok := h.readerFor(w, clusterName)
	if !ok {
		return
	}

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	rebooted, err := vm.SetCloudInitConfig(r.Context(), vm.CloudInitConfigDeps{
		Index: index, Actor: actor, ClusterName: clusterName, VMID: vmid,
		Reader: reader, Writer: writer, Audit: h.store, Refresher: h.refresher,
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

func (h *VMCloudInit) handleSSHKey(w http.ResponseWriter, r *http.Request) {
	clusterName, vmid, ok := parseCloudInitPath(r)
	if !ok {
		h.writeError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}

	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	var request cloudInitSSHKeyRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	if strings.TrimSpace(request.Key) == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_key", "a non-empty ssh public key is required")
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	reader, ok := h.readerFor(w, clusterName)
	if !ok {
		return
	}

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	user := request.User
	if user == "" {
		// Default to the cloud-init user so a bare key still lands on the
		// guest's primary account (mirrors how ciuser seeds the VM).
		if cfg, cfgErr := vm.GetCloudInitConfig(r.Context(), index, identity, clusterName, vmid, reader); cfgErr == nil && cfg.User != "" {
			user = cfg.User
		} else {
			user = "root"
		}
	}

	if err := vm.AddCloudInitSSHKey(r.Context(), vm.AddCloudInitSSHKeyDeps{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid,
		Reader: reader, Writer: writer, Audit: h.store,
	}, user, strings.TrimSpace(request.Key)); err != nil {
		h.writeDomainError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, cloudInitSSHKeyResponse{Status: "injected"})
}

func (h *VMCloudInit) handleSnippet(w http.ResponseWriter, r *http.Request) {
	h.serveRoute(w, r, h.getSnippet, h.putSnippet)
}

func (h *VMCloudInit) getSnippet(w http.ResponseWriter, r *http.Request, actor auth.Identity, clusterName string, vmid int) {
	index, ok := h.index(w, clusterName)
	if !ok {
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

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	reader, ok := h.readerFor(w, clusterName)
	if !ok {
		return
	}

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	if err := vm.SetCloudInitSnippet(r.Context(), vm.CloudInitSnippetDeps{
		Index: index, Actor: actor, ClusterName: clusterName, VMID: vmid,
		Reader: reader, Writer: writer, Store: h.store, Service: h.policy,
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
	case errors.Is(err, vm.ErrSSHKeyInvalid):
		h.writeError(w, http.StatusBadRequest, "invalid_key", err.Error())
	case errors.Is(err, cluster.ErrSSHKeyUserUnknown):
		h.writeError(w, http.StatusBadRequest, "ssh_user_unknown", "the cloud-init user does not exist on the guest")
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
