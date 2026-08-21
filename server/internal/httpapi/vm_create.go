package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
)

// VMCreate serves POST /api/v1/vms (the single creation endpoint for both
// simple and detailed modes — FR-001) and GET /api/v1/vm-create/catalog
// (FR-002). All validation lives in vm.Create; this handler only decodes,
// maps errors, and encodes.
type VMCreate struct {
	auth    *Auth
	store   *store.Store
	client  cluster.Client
	clients cluster.ClientProvider
	creator cluster.Creator
	pusher  vm.CloudInitPusher
	policy  *policy.Policy
	log     *slog.Logger
}

// NewVMCreate creates the handler. The creator is the cluster client's
// creation contract (allocation + async dispatch), separate from reads and
// from existing-VM writes (constitution IV). The pusher is the same cluster
// client's T08 cloud-init push contract, reused by vm.Create's template-apply
// step (FR-007) — never a second write mechanism.
func NewVMCreate(
	authHandler *Auth,
	st *store.Store,
	client cluster.Client,
	creator cluster.Creator,
	pusher vm.CloudInitPusher,
	log *slog.Logger,
	services ...*policy.Policy,
) *VMCreate {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	return &VMCreate{
		auth:    authHandler,
		store:   st,
		client:  client,
		creator: creator,
		pusher:  pusher,
		policy:  policyService,
		log:     log,
	}
}

// NewVMCreateWithRegistry creates a VM handler with cluster-aware catalog discovery.
func NewVMCreateWithRegistry(
	authHandler *Auth,
	st *store.Store,
	clients cluster.ClientProvider,
	creator cluster.Creator,
	pusher vm.CloudInitPusher,
	log *slog.Logger,
	services ...*policy.Policy,
) *VMCreate {
	handler := NewVMCreate(
		authHandler,
		st,
		nil,
		creator,
		pusher,
		log,
		services...,
	)
	handler.clients = clients

	return handler
}

type createResultDTO struct {
	Cluster             string `json:"cluster"`
	VMID                int    `json:"vmid"`
	Name                string `json:"name"`
	Node                string `json:"node"`
	UPID                string `json:"upid"`
	CloudInitTemplateID string `json:"cloudInitTemplateId,omitempty"`
	CloudInitPushError  string `json:"cloudInitPushError,omitempty"`
}

type catalogStorageDTO struct {
	Name string `json:"name"`
	Node string `json:"node"`
}

type catalogISODTO struct {
	Storage string `json:"storage"`
	File    string `json:"file"`
}

type catalogProfileDTO struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
}

type catalogCloudInitTemplateDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type catalogDTO struct {
	Cluster            string                        `json:"cluster"`
	Nodes              []string                      `json:"nodes"`
	Storages           []catalogStorageDTO           `json:"storages"`
	Bridges            []string                      `json:"bridges"`
	ISOs               []catalogISODTO               `json:"isos"`
	Profiles           []catalogProfileDTO           `json:"profiles"`
	CloudInitTemplates []catalogCloudInitTemplateDTO `json:"cloudInitTemplates"`
}

// ServeHTTP handles POST /api/v1/vms. Creation is asynchronous (FR-013):
// 202 means the task was accepted, not that the VM exists.
func (h *VMCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	var req vm.CreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeCreateError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	clusterName, creator, pusher, ok := h.resolveCreateTarget(w, req.Cluster)
	if !ok {
		return
	}

	result, err := vm.Create(r.Context(), identity, clusterName, req, vm.CreateDeps{
		Store:    h.store,
		Creator:  creator,
		Pusher:   pusher,
		Audit:    h.store,
		Log:      h.log,
		Services: []*policy.Policy{h.policy},
	})
	if err != nil {
		h.writeCreateFailure(w, err)
		return
	}

	h.writeCreateJSON(w, http.StatusAccepted, createResultDTO{
		Cluster:             result.Cluster,
		VMID:                result.VMID,
		Name:                result.Name,
		Node:                result.Node,
		UPID:                result.UPID,
		CloudInitTemplateID: result.CloudInitTemplateID,
		CloudInitPushError:  result.CloudInitPushError,
	})
}

// ServeCatalog handles GET /api/v1/vm-create/catalog. The catalog is the same
// for every user of a cluster (contracts behavioural rules) — no
// identity-specific filtering beyond requiring authentication.
func (h *VMCreate) ServeCatalog(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.Principal(r); err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, client, ok := h.resolveCatalogClient(w, r)
	if !ok {
		return
	}

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("catalog read failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	snap, err := client.Snapshot(r.Context())
	if err != nil {
		h.log.Error("storage discovery failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	profiles, err := catalog.Profiles(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("profile read failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	templates, err := catalog.CloudInitTemplates(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("cloudinit templates read failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := catalogDTO{
		Cluster:            clusterName,
		Nodes:              make([]string, 0, len(resources.Nodes)),
		Storages:           make([]catalogStorageDTO, 0, len(resources.Storages)),
		Bridges:            catalogBridgeNames(resources.Bridges),
		ISOs:               make([]catalogISODTO, 0, len(resources.ISOs)),
		Profiles:           make([]catalogProfileDTO, 0, len(profiles)),
		CloudInitTemplates: make([]catalogCloudInitTemplateDTO, 0, len(templates)),
	}
	for _, node := range resources.Nodes {
		dto.Nodes = append(dto.Nodes, node.Name)
	}

	for _, storage := range resources.Storages {
		if _, ok := vmCapableStorage(storage, snap.Storages); !ok {
			continue
		}

		dto.Storages = append(dto.Storages, catalogStorageDTO{Name: storage.Name, Node: storage.Node})
	}

	for _, iso := range resources.ISOs {
		dto.ISOs = append(dto.ISOs, catalogISODTO{Storage: iso.Storage, File: iso.File})
	}

	for _, profile := range profiles {
		dto.Profiles = append(dto.Profiles, catalogProfileDTO{
			ID:       profile.ID,
			Label:    profile.Label,
			CPUCores: profile.CPUCores,
			MemoryMB: profile.MemoryMB,
			DiskGB:   profile.DiskGB,
			Bus:      profile.Bus,
		})
	}

	// T18: catalog exposes only id+label per spec/contracts — never content.
	for _, tmpl := range templates {
		dto.CloudInitTemplates = append(dto.CloudInitTemplates, catalogCloudInitTemplateDTO{
			ID: tmpl.ID, Label: tmpl.Label,
		})
	}

	h.writeCreateJSON(w, http.StatusOK, dto)
}

func (h *VMCreate) resolveCatalogClient(w http.ResponseWriter, r *http.Request) (string, cluster.Client, bool) {
	clusterName, err := ResolveClusterParam(r, h.clients)
	if err != nil {
		code, message := clusterParamError(err)
		h.writeCreateError(w, http.StatusBadRequest, code, message)

		return "", nil, false
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		h.writeCreateError(w, http.StatusNotFound, "not_found", msgClusterNotFound)

		return "", nil, false
	}

	return clusterName, client, true
}

// resolveCreateTarget resolves the effective cluster name from req.Cluster
// (defaulting the same way ResolveClusterParam does for the catalog route)
// plus that cluster's own Creator and CloudInitPusher — without this, VM
// creation ran through the default cluster's client regardless of which
// cluster the request named.
func (h *VMCreate) resolveCreateTarget(w http.ResponseWriter, requestedCluster string) (string, cluster.Creator, vm.CloudInitPusher, bool) {
	clusterName, err := ResolveClusterValue(requestedCluster, h.clients)
	if err != nil {
		code, message := clusterParamError(err)
		h.writeCreateError(w, http.StatusBadRequest, code, message)

		return "", nil, nil, false
	}

	if h.clients == nil {
		return clusterName, h.creator, h.pusher, true
	}

	client, err := h.clients.Client(clusterName)
	if err != nil {
		h.writeCreateError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return "", nil, nil, false
	}

	creator, ok := client.(cluster.Creator)
	if !ok {
		h.log.Error("cluster client does not implement Creator", "component", "httpapi", "cluster", clusterName)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return "", nil, nil, false
	}

	pusher, ok := client.(vm.CloudInitPusher)
	if !ok {
		h.log.Error("cluster client does not implement CloudInitPusher", "component", "httpapi", "cluster", clusterName)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return "", nil, nil, false
	}

	return clusterName, creator, pusher, true
}

func (h *VMCreate) clientFor(clusterName string) (cluster.Client, error) {
	if h.clients != nil {
		return h.clients.Client(clusterName)
	}

	if h.client == nil {
		return nil, cluster.ErrClusterNotFound
	}

	return h.client, nil
}

func vmCapableStorage(storage catalog.Storage, available []cluster.Storage) (cluster.Storage, bool) {
	for _, candidate := range available {
		if candidate.Name == storage.Name && candidate.Node == storage.Node && cluster.IsVMCapableStorage(candidate) {
			return candidate, true
		}
	}

	return cluster.Storage{}, false
}

func catalogBridgeNames(bridges []catalog.Bridge) []string {
	names := make([]string, 0, len(bridges))
	seen := make(map[string]struct{}, len(bridges))

	for _, bridge := range bridges {
		if _, exists := seen[bridge.Name]; exists {
			continue
		}

		seen[bridge.Name] = struct{}{}
		names = append(names, bridge.Name)
	}

	return names
}

// writeCreateFailure maps vm.Create's sentinel errors to the contract's
// status codes and error codes.
func (h *VMCreate) writeCreateFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrAdminCannotCreate):
		h.writeCreateError(w, http.StatusForbidden, "admin_cannot_create", "administrators cannot create VMs")
	case errors.Is(err, vm.ErrNoPool):
		h.writeCreateError(w, http.StatusForbidden, "no_pool", "this account cannot own VMs")
	case errors.Is(err, policy.ErrQuotaExceeded):
		h.writeCreateError(w, http.StatusBadRequest, "quota_exceeded", err.Error())
	case errors.Is(err, policy.ErrGabaritExceeded):
		h.writeCreateError(w, http.StatusBadRequest, "gabarit_exceeded", err.Error())
	case errors.Is(err, policy.ErrNodeCapacityExceeded):
		h.writeCreateError(w, http.StatusBadRequest, "capacity_exceeded", err.Error())
	case errors.Is(err, vm.ErrInvalidName):
		h.writeCreateError(w, http.StatusBadRequest, "invalid_name", "name must be a valid hostname (lowercase alphanumeric and hyphen, no leading/trailing hyphen, max 63 chars)")
	case errors.Is(err, vm.ErrOutOfRange):
		h.writeCreateError(w, http.StatusBadRequest, "out_of_range", err.Error())
	case errors.Is(err, vm.ErrNotApproved):
		h.writeCreateError(w, http.StatusBadRequest, "not_approved", err.Error())
	case errors.Is(err, vm.ErrClusterCreate):
		h.log.Error("cluster create failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	default:
		h.log.Error("vm create failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMCreate) writeCreateJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

func (h *VMCreate) writeCreateError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
