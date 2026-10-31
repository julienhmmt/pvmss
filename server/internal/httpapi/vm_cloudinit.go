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
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
)

// VMCloudInit serves the per-VM cloud-init read (SSH user for the Connect
// tab) and the console-password action.
type VMCloudInit struct {
	projection   *inventory.Projection
	resolver     vm.ClusterIndexResolver
	auth         *Auth
	reader       cluster.CloudInitReader
	writer       cluster.Writer
	statusReader cluster.VMStatusReader
	clients      cluster.ClientProvider
	store        *store.Store
	refresher    vm.IndexRefresher
	refreshers   ClusterRefresherResolver
	policy       *policy.Policy
	log          *slog.Logger
}

// VMCloudInitDeps groups the shared dependencies for constructing a VMCloudInit
// handler. It collapses the seven positional parameters NewVMCloudInit used to
// take (SonarQube go:S107). Source and Clients are optional: when set (a
// multi-cluster deployment), every index load and cluster.Reader/Writer call
// below resolves per-request from the request's own :cluster path value
// instead of the single bound Projection/Reader/Writer.
type VMCloudInitDeps struct {
	Source       inventory.LookupSource
	Projection   *inventory.Projection
	Auth         *Auth
	Reader       cluster.CloudInitReader
	Writer       cluster.Writer
	StatusReader cluster.VMStatusReader
	Clients      cluster.ClientProvider
	Store        *store.Store
	Refresher    vm.IndexRefresher
	Log          *slog.Logger
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

	registry := depsSourceRegistry(deps.Source)
	resolver := vm.ClusterIndexResolver(registryResolver{registry: registry})
	refreshers := registryRefresherResolver{registry: registry}

	return &VMCloudInit{projection: deps.Projection, resolver: resolver, auth: deps.Auth, reader: deps.Reader, writer: deps.Writer, statusReader: deps.StatusReader, clients: deps.Clients, store: deps.Store, refresher: deps.Refresher, refreshers: refreshers, policy: policyService, log: deps.Log}
}

// statusReaderFor resolves the cluster.VMStatusReader for clusterName, falling
// back to the single-cluster reader when per-cluster resolution is
// unavailable - the same fallback rule VMStatusBatch applies.
func (h *VMCloudInit) statusReaderFor(clusterName string) cluster.VMStatusReader {
	if h.clients == nil {
		return h.statusReader
	}

	reader, err := resolveCapability(h.clients, h.statusReader, clusterName, "VMStatusReader")
	if err != nil {
		return h.statusReader
	}

	return reader
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

// refresherFor resolves the vm.IndexRefresher for clusterName. A missing
// refresher must not fail a write already applied on the cluster - so it
// never writes an HTTP error. When the per-cluster resolver is unset
// (single-cluster mode) or the cluster is unknown, it returns the fallback
// refresher and logs a warning. The result is never nil when the fallback
// is non-nil.
func (h *VMCloudInit) refresherFor(clusterName string) vm.IndexRefresher {
	if h.refreshers == nil {
		return h.refresher
	}

	refresher, err := h.refreshers.RefresherFor(clusterName)
	if err != nil {
		h.log.Warn("refresher not found for cluster, using fallback", "component", "httpapi", "cluster", clusterName, "error", err)
		return h.refresher
	}

	return refresher
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
// ServeHTTP dispatches the cloud-init read and console-password routes by path suffix.
func (h *VMCloudInit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/console-password") {
		h.handleConsolePassword(w, r)
		return
	}

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

// consolePasswordResponse carries the generated password once for display.
// The password is not persisted, not logged, and not recorded in the audit
// trail.
type consolePasswordResponse struct {
	Password string `json:"password"`
}

func (h *VMCloudInit) handleConsolePassword(w http.ResponseWriter, r *http.Request) {
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

	password, err := vm.SetConsolePassword(r.Context(), vm.ConsolePasswordDeps{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid,
		Reader: reader, Writer: writer, Audit: h.store, Refresher: h.refresherFor(clusterName),
		StatusReader: h.statusReaderFor(clusterName),
	})
	if err != nil {
		h.writeDomainError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, consolePasswordResponse{Password: password})
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
	case h.writeGuestAgentError(w, err):
		// Already written by the helper (password-path errors).
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

// msgGuestAgentUnreachable is what a client sees when the guest agent never
// answers; the Proxmox URL and transport error stay in the access log.
const msgGuestAgentUnreachable = "guest agent did not answer in time; is qemu-guest-agent installed and running?"

// writeGuestAgentError maps the password-path errors introduced by tickets 02
// and 05. It writes the response and reports true when err is one of them.
func (h *VMCloudInit) writeGuestAgentError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, vm.ErrNoCloudInitUser):
		h.writeError(w, http.StatusBadRequest, "no_cloudinit_user", "no cloud-init user is defined on this VM; set one before setting a password")
	case errors.Is(err, vm.ErrGuestAgentDisabled):
		h.writeError(w, http.StatusConflict, "guest_agent_disabled", "the QEMU guest agent is not enabled on this VM")
	case errors.Is(err, vm.ErrVMNotRunning):
		h.writeError(w, http.StatusConflict, "vm_not_running", "start the VM first")
	case errors.Is(err, vm.ErrGuestAgentUnreachable):
		SetError(w, err)
		h.writeError(w, http.StatusGatewayTimeout, "guest_agent_unreachable", msgGuestAgentUnreachable)
	default:
		return false
	}

	return true
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
