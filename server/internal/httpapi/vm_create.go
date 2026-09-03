package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
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
	auth             *Auth
	store            *store.Store
	client           cluster.Client
	clients          cluster.ClientProvider
	creator          cluster.Creator
	pusher           vm.CloudInitPusher
	policy           *policy.Policy
	log              *slog.Logger
	trustedProxyHops int
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

// SetTrustedProxyHops configures how many X-Forwarded-For hops to trust for
// client IP extraction used in audit log entries.
func (h *VMCreate) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
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

// catalogBridgeDTO is one approved bridge on one node — bridge approval is
// per-node (like storage), so the client needs the node to both label the
// option and filter to the VM's chosen node.
type catalogBridgeDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Comment string `json:"comment,omitempty"`
}

type catalogISODTO struct {
	Storage string `json:"storage"`
	Node    string `json:"node"`
	File    string `json:"file"`
}

type catalogProfileDTO struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Sockets  int    `json:"sockets"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
}

type catalogCloudInitTemplateDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// catalogTemplateDTO is one approved Proxmox template (US2/issue-02). The
// VMID is the Proxmox VMID of the template; the node determines where the
// clone lands (D2b: cross-node clone is forbidden, so the UI hides the node
// selector when a template is chosen). CloudInitCapable signals the UI that
// the template supports cloud-init. DiskSizeGB lets the UI show the minimum
// disk size (reductions are rejected). DiskStorage is the template disk's
// source storage — the UI uses it to warn when the chosen target storage
// forces a full copy (buildCloneSpec's rule).
type catalogTemplateDTO struct {
	VMID             int    `json:"vmid"`
	Node             string `json:"node"`
	Name             string `json:"name"`
	CloudInitCapable bool   `json:"cloudInitCapable"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	DiskStorage      string `json:"diskStorage"`
}

type catalogTagDTO struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// catalogGabaritDTO is the administrator-editable per-VM size ceiling (T12
// gabarit) — the client uses it to validate hardware/disk fields before
// submit and to show the user what they're allowed, not just what failed.
type catalogGabaritDTO struct {
	MaxSockets       int `json:"maxSockets"`
	MaxCores         int `json:"maxCores"`
	MaxMemoryMB      int `json:"maxMemoryMB"`
	MaxDiskPerVMGB   int `json:"maxDiskPerVMGB"`
	MaxNetworkCards  int `json:"maxNetworkCards"`
	MaxSnapshots     int `json:"maxSnapshots"`
	IsolationVLANTag int `json:"isolationVlanTag"`
}

// catalogQuotaDTO is the caller's own VM count against the cluster's
// per-user allowance. Allowed is -1 for unlimited (policy.Quota contract).
type catalogQuotaDTO struct {
	Used    int `json:"used"`
	Allowed int `json:"allowed"`
}

// catalogNodeCapacityDTO is one approved node's configured aggregate
// capacité, live usage, and physical facts (policy.Capacity). Omitted from
// the response for a node with no capacité configured (all-zero row).
type catalogNodeCapacityDTO struct {
	Node          string `json:"node"`
	MaxVMs        int    `json:"maxVMs"`
	MaxVCPUs      int    `json:"maxVCPUs"`
	MaxRAMGB      int    `json:"maxRAMGB"`
	MaxDiskGB     int    `json:"maxDiskGB"`
	UsedVMs       int    `json:"usedVMs"`
	UsedVCPUs     int    `json:"usedVCPUs"`
	UsedRAMGB     int    `json:"usedRAMGB"`
	UsedDiskGB    int    `json:"usedDiskGB"`
	PhysicalVCPUs int    `json:"physicalVCPUs"`
	PhysicalRAMGB int    `json:"physicalRAMGB"`
}

type catalogDTO struct {
	Cluster            string                        `json:"cluster"`
	Nodes              []string                      `json:"nodes"`
	Storages           []catalogStorageDTO           `json:"storages"`
	Bridges            []catalogBridgeDTO            `json:"bridges"`
	ISOs               []catalogISODTO               `json:"isos"`
	Profiles           []catalogProfileDTO           `json:"profiles"`
	Templates          []catalogTemplateDTO          `json:"templates"`
	CloudInitTemplates []catalogCloudInitTemplateDTO `json:"cloudInitTemplates"`
	Tags               []catalogTagDTO               `json:"tags"`
	Gabarit            *catalogGabaritDTO            `json:"gabarit,omitempty"`
	Quota              *catalogQuotaDTO              `json:"quota,omitempty"`
	NodeCapacities     []catalogNodeCapacityDTO      `json:"nodeCapacities,omitempty"`
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
		h.writeCreateError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	target, ok := h.resolveCreateTarget(w, req.Cluster)
	if !ok {
		return
	}

	ctx := policy.ContextWithAuditIP(r.Context(), clientIP(r, h.trustedProxyHops))

	result, err := vm.Create(ctx, identity, target.clusterName, req, vm.CreateDeps{
		Store:     h.store,
		Creator:   target.creator,
		Pusher:    target.pusher,
		Writer:    target.writer,
		FreeSpace: target.freeSpace,
		Snippets:  target.snippets,
		Audit:     h.store,
		Log:       h.log,
		Services:  []*policy.Policy{h.policy},
		Templates: target.templates,
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

// catalogData is the raw catalog payload loaded from the store and the live
// cluster before it is shaped into the response DTO.
type catalogData struct {
	resources        catalog.Resources
	snap             cluster.Snapshot
	bridges          []cluster.Bridge
	profiles         []catalog.Profile
	templates        []catalog.CloudInitTemplate
	proxmoxTemplates []catalog.Template
	tags             []catalog.TagWithCount
}

// loadCatalogData fetches every catalog slice from the store and the live
// cluster. Errors are wrapped with the operation that failed so the handler
// can log a single, actionable message.
func (h *VMCreate) loadCatalogData(ctx context.Context, client cluster.Client, clusterName string) (catalogData, error) {
	var data catalogData

	resources, err := catalog.ApprovedResources(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("approved resources: %w", err)
	}

	data.resources = resources

	snap, err := client.Snapshot(ctx)
	if err != nil {
		return catalogData{}, fmt.Errorf("storage discovery: %w", err)
	}

	data.snap = snap

	bridges, err := client.ListBridges(ctx)
	if err != nil {
		return catalogData{}, fmt.Errorf("bridge discovery: %w", err)
	}

	data.bridges = bridges

	profiles, err := catalog.Profiles(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("profiles: %w", err)
	}

	data.profiles = profiles

	templates, err := catalog.CloudInitTemplates(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("cloudinit templates: %w", err)
	}

	data.templates = templates

	// US2/issue-02: approved Proxmox templates (clone source).
	proxmoxTemplates, err := catalog.Templates(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("proxmox templates: %w", err)
	}

	data.proxmoxTemplates = proxmoxTemplates

	// Admin-created tags only (FR-014/FR-015 surface) — the mandatory pvmss
	// tag is added server-side and never offered as a user choice here.
	tags, err := catalog.ListTags(ctx, h.store, nil, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("tags: %w", err)
	}

	data.tags = tags

	return data, nil
}

// buildCatalogDTO maps the raw catalog data into the response contract.
func buildCatalogDTO(clusterName string, data catalogData) catalogDTO {
	dto := catalogDTO{
		Cluster:            clusterName,
		Nodes:              make([]string, 0, len(data.resources.Nodes)),
		Storages:           make([]catalogStorageDTO, 0, len(data.resources.Storages)),
		Bridges:            catalogBridgeDTOs(data.resources.Bridges, data.bridges),
		ISOs:               make([]catalogISODTO, 0, len(data.resources.ISOs)),
		Profiles:           make([]catalogProfileDTO, 0, len(data.profiles)),
		Templates:          make([]catalogTemplateDTO, 0, len(data.proxmoxTemplates)),
		CloudInitTemplates: make([]catalogCloudInitTemplateDTO, 0, len(data.templates)),
		Tags:               make([]catalogTagDTO, 0, len(data.tags)),
	}

	for _, node := range data.resources.Nodes {
		dto.Nodes = append(dto.Nodes, node.Name)
	}

	for _, tag := range data.tags {
		if tag.Protected {
			continue
		}

		dto.Tags = append(dto.Tags, catalogTagDTO{Name: tag.Name, Color: tag.Color})
	}

	for _, storage := range data.resources.Storages {
		if _, ok := vmCapableStorage(storage, data.snap.Storages); !ok {
			continue
		}

		dto.Storages = append(dto.Storages, catalogStorageDTO{Name: storage.Name, Node: storage.Node})
	}

	for _, iso := range data.resources.ISOs {
		dto.ISOs = append(dto.ISOs, catalogISODTO{Storage: iso.Storage, Node: iso.Node, File: iso.File})
	}

	for _, profile := range data.profiles {
		dto.Profiles = append(dto.Profiles, catalogProfileDTO{
			ID:       profile.ID,
			Label:    profile.Label,
			Sockets:  profile.Sockets,
			CPUCores: profile.CPUCores,
			MemoryMB: profile.MemoryMB,
			DiskGB:   profile.DiskGB,
			Bus:      profile.Bus,
		})
	}

	// T18: catalog exposes only id+label per spec/contracts — never content.
	for _, tmpl := range data.templates {
		dto.CloudInitTemplates = append(dto.CloudInitTemplates, catalogCloudInitTemplateDTO{
			ID: tmpl.ID, Label: tmpl.Label,
		})
	}

	// US2/issue-02: approved Proxmox templates (clone source).
	for _, tmpl := range data.proxmoxTemplates {
		dto.Templates = append(dto.Templates, catalogTemplateDTO{
			VMID:             tmpl.VMID,
			Node:             tmpl.Node,
			Name:             tmpl.Name,
			CloudInitCapable: tmpl.CloudInitCapable,
			DiskSizeGB:       tmpl.DiskSizeGB,
			DiskStorage:      tmpl.DiskStorage,
		})
	}

	return dto
}

// ServeCatalog handles GET /api/v1/vm-create/catalog. The catalog is the same
// for every user of a cluster (contracts behavioural rules) — no
// identity-specific filtering beyond requiring authentication.
func (h *VMCreate) ServeCatalog(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, client, ok := h.resolveCatalogClient(w, r)
	if !ok {
		return
	}

	data, err := h.loadCatalogData(r.Context(), client, clusterName)
	if err != nil {
		h.log.Error("catalog data load failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := buildCatalogDTO(clusterName, data)

	if err := h.attachLimits(r.Context(), &dto, clusterName, identity); err != nil {
		h.log.Error("policy read failed", "component", "httpapi", "error", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.writeCreateJSON(w, http.StatusOK, dto)
}

// attachLimits fills the catalog's gabarit/quota/nodeCapacities so the
// detailed-mode wizard can show what the user is allowed and validate
// hardware/disk fields client-side before the server re-checks them
// (constitution VI: client bounds are a convenience only).
func (h *VMCreate) attachLimits(ctx context.Context, dto *catalogDTO, clusterName string, identity auth.Identity) error {
	if h.policy == nil {
		return nil
	}

	gabarit, err := h.policy.Gabarit(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("read gabarit: %w", err)
	}

	dto.Gabarit = &catalogGabaritDTO{
		MaxSockets: gabarit.MaxSockets, MaxCores: gabarit.MaxCores, MaxMemoryMB: gabarit.MaxMemoryMB,
		MaxDiskPerVMGB: gabarit.MaxDiskPerVMGB, MaxNetworkCards: gabarit.MaxNetworkCards,
		MaxSnapshots: gabarit.MaxSnapshots, IsolationVLANTag: gabarit.IsolationVLANTag,
	}

	quota, err := h.policy.Quota(ctx, clusterName, identity)
	if err != nil {
		return fmt.Errorf("read quota: %w", err)
	}

	dto.Quota = &catalogQuotaDTO{Used: quota.Used, Allowed: quota.Allowed}

	dto.NodeCapacities = make([]catalogNodeCapacityDTO, 0, len(dto.Nodes))

	for _, node := range dto.Nodes {
		capacity, err := h.policy.NodeCapacity(ctx, clusterName, node)
		if err != nil {
			return fmt.Errorf("read node capacity for %q: %w", node, err)
		}

		if capacity.MaxVMs == 0 && capacity.MaxVCPUs == 0 && capacity.MaxRAMGB == 0 {
			continue // no capacité configured for this node — nothing to show
		}

		dto.NodeCapacities = append(dto.NodeCapacities, catalogNodeCapacityDTO{
			Node: node, MaxVMs: capacity.MaxVMs, MaxVCPUs: capacity.MaxVCPUs, MaxRAMGB: capacity.MaxRAMGB,
			MaxDiskGB: capacity.MaxDiskGB, UsedVMs: capacity.UsedVMs, UsedVCPUs: capacity.UsedVCPUs,
			UsedRAMGB: capacity.UsedRAMGB, UsedDiskGB: capacity.UsedDiskGB,
			PhysicalVCPUs: capacity.PhysicalVCPUs, PhysicalRAMGB: capacity.PhysicalRAMGB,
		})
	}

	return nil
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

// createTarget bundles the per-cluster capabilities the create path needs,
// resolved from the request's own cluster (never the default client).
type createTarget struct {
	clusterName string
	creator     cluster.Creator
	pusher      vm.CloudInitPusher
	writer      vm.HardwareUpdater
	freeSpace   vm.FreeSpaceChecker
	snippets    vm.SnippetStorageFinder
	templates   vm.TemplateReader
}

// resolveCreateTarget resolves the effective cluster name from req.Cluster
// (defaulting the same way ResolveClusterParam does for the catalog route)
// plus that cluster's own Creator, CloudInitPusher, HardwareUpdater, and
// SnippetStorageFinder — without this, VM creation ran through the default
// cluster's client regardless of which cluster the request named. The
// HardwareUpdater is needed for post-clone configuration (US2/issue-02); the
// SnippetStorageFinder for the plan-time snippet storage resolution (ticket 04).
func (h *VMCreate) resolveCreateTarget(w http.ResponseWriter, requestedCluster string) (createTarget, bool) {
	clusterName, err := ResolveClusterValue(requestedCluster, h.clients)
	if err != nil {
		code, message := clusterParamError(err)
		h.writeCreateError(w, http.StatusBadRequest, code, message)

		return createTarget{}, false
	}

	if h.clients == nil {
		writer, _ := h.creator.(vm.HardwareUpdater)
		freeSpace, _ := h.creator.(vm.FreeSpaceChecker)
		snippets, _ := h.creator.(vm.SnippetStorageFinder)
		templates, _ := h.creator.(vm.TemplateReader)

		return createTarget{clusterName: clusterName, creator: h.creator, pusher: h.pusher, writer: writer, freeSpace: freeSpace, snippets: snippets, templates: templates}, true
	}

	client, err := h.clients.Client(clusterName)
	if err != nil {
		h.writeCreateError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return createTarget{}, false
	}

	creator, ok := client.(cluster.Creator)
	if !ok {
		h.log.Error("cluster client does not implement Creator", "component", "httpapi", "cluster", clusterName)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	pusher, ok := client.(vm.CloudInitPusher)
	if !ok {
		h.log.Error("cluster client does not implement CloudInitPusher", "component", "httpapi", "cluster", clusterName)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	writer, ok := client.(vm.HardwareUpdater)
	if !ok {
		h.log.Error("cluster client does not implement HardwareUpdater", "component", "httpapi", "cluster", clusterName)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	freeSpace, ok := client.(vm.FreeSpaceChecker)
	if !ok {
		h.log.Error("cluster client does not implement FreeSpaceChecker", "component", "httpapi", "cluster", clusterName)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	// Optional capability: a client without FindSnippetStorage only blocks
	// cloud-init template requests (planCreate refuses before VMID
	// allocation), not plain ISO creations.
	snippets, _ := client.(vm.SnippetStorageFinder)

	// Optional capability: the clone-time freshness backstop (T17). A client
	// without TemplateByVMID skips the backstop.
	templates, _ := client.(vm.TemplateReader)

	return createTarget{clusterName: clusterName, creator: creator, pusher: pusher, writer: writer, freeSpace: freeSpace, snippets: snippets, templates: templates}, true
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

// catalogBridgeDTOs dedupes by (name, node) — the same bridge name can be
// approved on more than one node, and each is a distinct, independently
// selectable option (bridge approval is per-node, like storage). live carries
// the cluster's current network config, which is where the description
// (Proxmox "comments" field) actually lives — catalog_bridges only stores
// the approval, not the comment.
func catalogBridgeDTOs(bridges []catalog.Bridge, live []cluster.Bridge) []catalogBridgeDTO {
	type key struct{ name, node string }

	commentByKey := make(map[key]string, len(live))
	for _, bridge := range live {
		commentByKey[key{bridge.Name, bridge.Node}] = bridge.Comment
	}

	out := make([]catalogBridgeDTO, 0, len(bridges))
	seen := make(map[key]struct{}, len(bridges))

	for _, bridge := range bridges {
		k := key{bridge.Name, bridge.Node}
		if _, exists := seen[k]; exists {
			continue
		}

		seen[k] = struct{}{}
		out = append(out, catalogBridgeDTO{Name: bridge.Name, Node: bridge.Node, Comment: commentByKey[k]})
	}

	return out
}

// writeCreateFailure maps vm.Create's sentinel errors to the contract's
// status codes and error codes.
func (h *VMCreate) writeCreateFailure(w http.ResponseWriter, err error) {
	if status, code, message, ok := mapCreateError(err); ok {
		if code == "cluster_error" {
			h.log.Error("cluster create failed", "component", "httpapi", "error", err)
		}

		h.writeCreateError(w, status, code, message)

		return
	}

	h.log.Error("vm create failed", "component", "httpapi", "error", err)
	h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

// createErrorMapping pairs a sentinel error with its HTTP status, error code,
// and message. A nil message means "use err.Error()" (the sentinel carries a
// dynamic detail string).
type createErrorMapping struct {
	err     error
	status  int
	code    string
	message string // empty → err.Error()
}

// createErrorMappings is the table writeCreateFailure consults. Order matters
// only for errors.Is precedence, which is identity-based here.
var createErrorMappings = []createErrorMapping{
	{vm.ErrAdminCannotCreate, http.StatusForbidden, "admin_cannot_create", "administrators cannot create VMs"},
	{vm.ErrNoPool, http.StatusForbidden, "no_pool", "this account cannot own VMs"},
	{policy.ErrQuotaExceeded, http.StatusBadRequest, "quota_exceeded", ""},
	{policy.ErrGabaritExceeded, http.StatusBadRequest, "gabarit_exceeded", ""},
	{policy.ErrNodeCapacityExceeded, http.StatusBadRequest, "capacity_exceeded", ""},
	{vm.ErrInvalidName, http.StatusBadRequest, "invalid_name", "name must be a valid hostname (lowercase alphanumeric and hyphen, no leading/trailing hyphen, max 63 chars)"},
	{vm.ErrNameTaken, http.StatusBadRequest, "name_taken", ""},
	{vm.ErrOutOfRange, http.StatusBadRequest, "out_of_range", ""},
	{vm.ErrNotApproved, http.StatusBadRequest, "not_approved", ""},
	{vm.ErrInvalidSource, http.StatusBadRequest, "invalid_source", ""},
	{vm.ErrInvalidRequest, http.StatusBadRequest, codeInvalidRequest, ""},
	{vm.ErrDiskReduction, http.StatusBadRequest, "disk_reduction", ""},
	{vm.ErrInsufficientDiskSpace, http.StatusBadRequest, "insufficient_disk_space", ""},
	{vm.ErrNoSnippetStorage, http.StatusBadRequest, "no_snippet_storage", ""},
	{vm.ErrClusterCreate, http.StatusBadGateway, "cluster_error", msgClusterRejected},
}

// mapCreateError returns the HTTP status, code, and message for a known
// sentinel error, or (0, "", "", false) for an unrecognized error.
func mapCreateError(err error) (int, string, string, bool) {
	for _, m := range createErrorMappings {
		if errors.Is(err, m.err) {
			msg := m.message
			if msg == "" {
				msg = err.Error()
			}

			return m.status, m.code, msg, true
		}
	}

	return 0, "", "", false
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
