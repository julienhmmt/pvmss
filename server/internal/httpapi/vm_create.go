package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
)

// VmCreate serves POST /api/v1/vms (the single creation endpoint for both
// simple and detailed modes — FR-001) and GET /api/v1/vm-create/catalog
// (FR-002). All validation lives in vm.Create; this handler only decodes,
// maps errors, and encodes.
type VmCreate struct {
	auth    *Auth
	store   *store.Store
	creator cluster.Creator
	log     *slog.Logger
}

// NewVmCreate creates the handler. The creator is the cluster client's
// creation contract (allocation + async dispatch), separate from reads and
// from existing-VM writes (constitution IV).
func NewVmCreate(authHandler *Auth, st *store.Store, creator cluster.Creator, log *slog.Logger) *VmCreate {
	return &VmCreate{auth: authHandler, store: st, creator: creator, log: log}
}

type createResultDTO struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Name    string `json:"name"`
	Node    string `json:"node"`
	UPID    string `json:"upid"`
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

type catalogDTO struct {
	Cluster  string              `json:"cluster"`
	Nodes    []string            `json:"nodes"`
	Storages []catalogStorageDTO `json:"storages"`
	Bridges  []string            `json:"bridges"`
	ISOs     []catalogISODTO     `json:"isos"`
	Profiles []catalogProfileDTO `json:"profiles"`
}

// ServeHTTP handles POST /api/v1/vms. Creation is asynchronous (FR-013):
// 202 means the task was accepted, not that the VM exists.
func (h *VmCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	var req vm.VMCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeCreateError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	result, err := vm.Create(r.Context(), identity, req.Cluster, req, h.store, h.creator, h.store, h.log)
	if err != nil {
		h.writeCreateFailure(w, err)
		return
	}
	h.writeCreateJSON(w, http.StatusAccepted, createResultDTO{
		Cluster: result.Cluster,
		VMID:    result.VMID,
		Name:    result.Name,
		Node:    result.Node,
		UPID:    result.UPID,
	})
}

// ServeCatalog handles GET /api/v1/vm-create/catalog. The catalog is the same
// for every user of a cluster (contracts behavioural rules) — no
// identity-specific filtering beyond requiring authentication.
func (h *VmCreate) ServeCatalog(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.Principal(r); err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	clusterName := r.URL.Query().Get("cluster")
	if clusterName == "" {
		clusterName = "default"
	}
	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("catalog read failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	profiles, err := catalog.Profiles(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("profile read failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	dto := catalogDTO{
		Cluster:  clusterName,
		Nodes:    make([]string, 0, len(resources.Nodes)),
		Storages: make([]catalogStorageDTO, 0, len(resources.Storages)),
		Bridges:  resources.Bridges,
		ISOs:     make([]catalogISODTO, 0, len(resources.ISOs)),
		Profiles: make([]catalogProfileDTO, 0, len(profiles)),
	}
	for _, node := range resources.Nodes {
		dto.Nodes = append(dto.Nodes, node.Name)
	}
	for _, storage := range resources.Storages {
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
	h.writeCreateJSON(w, http.StatusOK, dto)
}

// writeCreateFailure maps vm.Create's sentinel errors to the contract's
// status codes and error codes.
func (h *VmCreate) writeCreateFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrNoPool):
		h.writeCreateError(w, http.StatusForbidden, "no_pool", "this account cannot own VMs")
	case errors.Is(err, vm.ErrInvalidName):
		h.writeCreateError(w, http.StatusBadRequest, "invalid_name", "name must be a valid hostname (lowercase alphanumeric and hyphen, no leading/trailing hyphen, max 63 chars)")
	case errors.Is(err, vm.ErrOutOfRange):
		h.writeCreateError(w, http.StatusBadRequest, "out_of_range", err.Error())
	case errors.Is(err, vm.ErrNotApproved):
		h.writeCreateError(w, http.StatusBadRequest, "not_approved", err.Error())
	case errors.Is(err, vm.ErrClusterCreate):
		h.log.Error("cluster create failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")
	default:
		h.log.Error("vm create failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *VmCreate) writeCreateJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

func (h *VmCreate) writeCreateError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
