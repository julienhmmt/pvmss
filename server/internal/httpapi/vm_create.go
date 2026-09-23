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
	"time"
)

// createWriteDeadlineMargin covers the post-clone/create configuration steps
// (hardware overrides, disk resize, cloud-init push) that run after
// vm.WaitCreateTask returns, before the response is written.
const createWriteDeadlineMargin = 2 * time.Minute

// VMCreate serves POST /api/v1/vms (the single creation endpoint for both
// simple and detailed modes) and GET /api/v1/vm-create/catalog.
// All validation lives in vm.Create; this handler only decodes,
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
	// refreshers, when set, rebuilds the target cluster's inventory right
	// after a successful create so the VM is in /api/v1/vms by the time
	// the wizard navigates to the list - instead of depending on the task
	// tray's poll landing after the list load.
	refreshers ClusterRefresherResolver
}

// SetInventoryRefreshers wires the post-create inventory refresh.
func (h *VMCreate) SetInventoryRefreshers(refreshers ClusterRefresherResolver) {
	h.refreshers = refreshers
}

// refreshAfterCreate rebuilds clusterName's projection. Every create path
// has already waited for its Proxmox task, so the VM exists now.
// Best-effort: a failure only delays visibility to the next cycle.
func (h *VMCreate) refreshAfterCreate(ctx context.Context, clusterName string) {
	if h.refreshers == nil {
		return
	}

	refresher, err := h.refreshers.RefresherFor(clusterName)
	if err != nil {
		return
	}

	if _, err := refresher.Refresh(ctx); err != nil {
		h.log.Warn("post-create inventory refresh failed", "component", "httpapi", "cluster", clusterName, "error", err)
	}
}

// NewVMCreate creates the handler. The creator is the cluster client's
// creation contract (allocation + async dispatch), separate from reads and
// from existing-VM writes. The pusher is the same cluster
// client's cloud-init push contract, reused by vm.Create's template-apply
// step - never a second write mechanism.
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
	// FromImage is true when the VM was created from a cloud image
	// the create summary warns that SSH
	// is the only access until a console password is set.
	FromImage bool `json:"fromImage,omitempty"`
}

type catalogStorageDTO struct {
	Name string `json:"name"`
	Node string `json:"node"`
}

// catalogBridgeDTO is one approved bridge on one node - bridge approval is
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

// catalogImageDTO is one approved cloud image (import-from source). SizeBytes
// lets the UI enforce the minimum disk size (reductions are rejected).
type catalogImageDTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
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

// catalogTemplateDTO is one approved Proxmox template. The
// VMID is the Proxmox VMID of the template; the node determines where the
// clone lands (cross-node clone is forbidden, so the UI hides the node selector when a template
// is chosen). CloudInitCapable signals the UI that
// the template supports cloud-init. DiskSizeGB lets the UI show the minimum
// disk size (reductions are rejected). DiskStorage is the template disk's
// source storage - the UI uses it to warn when the chosen target storage
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

// catalogGabaritDTO is the administrator-editable per-VM size ceiling (gabarit) - the client
// uses it to validate hardware/disk fields before
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
	Images             []catalogImageDTO             `json:"images"`
	Profiles           []catalogProfileDTO           `json:"profiles"`
	Templates          []catalogTemplateDTO          `json:"templates"`
	CloudInitTemplates []catalogCloudInitTemplateDTO `json:"cloudInitTemplates"`
	// CloudInitWriteEnabled reports whether this cluster has a snippet write
	// target configured (Admin › Clusters): cloud-init documents can be
	// written, so the wizard may offer the document picker.
	CloudInitWriteEnabled bool `json:"cloudInitWriteEnabled"`
	// CloudInitBaselineID is the document id of the standalone baseline
	// (published for image VMs with no template). The web client compares a
	// VM's document id against it instead of hardcoding the value.
	CloudInitBaselineID string                   `json:"cloudInitBaselineId"`
	Tags                []catalogTagDTO          `json:"tags"`
	Gabarit             *catalogGabaritDTO       `json:"gabarit,omitempty"`
	Quota               *catalogQuotaDTO         `json:"quota,omitempty"`
	NodeCapacities      []catalogNodeCapacityDTO `json:"nodeCapacities,omitempty"`
}

// ServeHTTP handles POST /api/v1/vms. Creation is asynchronous:
// 202 means the task was accepted, not that the VM exists.
func (h *VMCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	// Creation blocks on vm.WaitCreateTask (up to vm.MaxCreateTaskWait) before
	// writing any response - the server's global WriteTimeout (10s,
	// cmd/pvmss/main.go) is nowhere near enough for a template clone or image
	// import. Extend just this response's write deadline so a slow clone
	// still reaches the client instead of the connection dying underneath a
	// creation that actually succeeded (report: VM created in Proxmox, PVMSS
	// showed "Échec de la création").
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(vm.MaxCreateTaskWait + createWriteDeadlineMargin))
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

	h.refreshAfterCreate(r.Context(), target.clusterName)

	h.writeCreateJSON(w, http.StatusAccepted, createResultDTO{
		Cluster:             result.Cluster,
		VMID:                result.VMID,
		Name:                result.Name,
		Node:                result.Node,
		UPID:                result.UPID,
		CloudInitTemplateID: result.CloudInitTemplateID,
		CloudInitPushError:  result.CloudInitPushError,
		FromImage:           result.FromImage,
	})
}

// catalogData is the raw catalog payload loaded from the store and the live
// cluster before it is shaped into the response DTO.
type catalogData struct {
	resources        catalog.Resources
	snap             cluster.Snapshot
	bridges          []cluster.Bridge
	isos             []cluster.ISOImage
	images           []catalog.Image
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

	if err := loadClusterDiscovery(ctx, client, &data); err != nil {
		return catalogData{}, err
	}

	profiles, err := catalog.Profiles(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("profiles: %w", err)
	}

	data.profiles = profiles

	templates, err := catalog.CloudInitTemplates(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("cloudinit templates: %w", err)
	}

	// Offer only templates published on the cluster: an enabled template
	// with no file behind it would be refused at create time anyway.
	publications, err := h.store.ListCloudInitPublications(ctx, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("cloudinit publications: %w", err)
	}

	for _, t := range templates {
		if _, ok := publications[t.ID]; ok {
			data.templates = append(data.templates, t)
		}
	}

	// Approved Proxmox templates (clone source).
	proxmoxTemplates, err := catalog.Templates(ctx, h.store, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("proxmox templates: %w", err)
	}

	data.proxmoxTemplates = proxmoxTemplates

	// Admin-created tags only - the mandatory pvmss
	// tag is added server-side and never offered as a user choice here.
	tags, err := catalog.ListTags(ctx, h.store, nil, clusterName)
	if err != nil {
		return catalogData{}, fmt.Errorf("tags: %w", err)
	}

	data.tags = tags

	return data, nil
}

// loadClusterDiscovery fills the live-discovery half of the catalog: the
// storage snapshot, bridges, ISOs, and cloud images the cluster reports.
func loadClusterDiscovery(ctx context.Context, client cluster.Client, data *catalogData) error {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("storage discovery: %w", err)
	}

	data.snap = snap

	bridges, err := client.ListBridges(ctx)
	if err != nil {
		return fmt.Errorf("bridge discovery: %w", err)
	}

	data.bridges = bridges

	isos, err := client.ListISOs(ctx)
	if err != nil {
		return fmt.Errorf("iso discovery: %w", err)
	}

	data.isos = isos

	images, err := client.ListCloudImages(ctx)
	if err != nil {
		return fmt.Errorf("image discovery: %w", err)
	}

	for _, image := range images {
		data.images = append(data.images, catalog.Image{Storage: image.Storage, Node: image.Node, File: image.File, SizeBytes: image.SizeBytes})
	}

	return nil
}

// buildCatalogDTO maps the raw catalog data into the response contract.
// Approved resources are filtered against live discovery: a node, storage,
// bridge, or ISO that Proxmox no longer reports is dropped from the user-facing
// catalog even if its approval row still exists in the store (the admin list
// surfaces those orphans for manual cleanup, and enabled orphans are
// auto-removed there).
func buildCatalogDTO(clusterName string, data catalogData, cloudInitWriteEnabled bool) catalogDTO {
	return catalogDTO{
		Cluster:               clusterName,
		Nodes:                 catalogNodeNames(data.resources.Nodes, data.snap),
		Storages:              catalogStorageDTOs(data.resources.Storages, data.snap.Storages),
		Bridges:               catalogBridgeDTOs(data.resources.Bridges, data.bridges),
		ISOs:                  catalogFileDTOs(data.resources.ISOs, catalogISOKey, liveISOKeys(data.isos), catalogISOView),
		Images:                catalogImageDTOs(data),
		Profiles:              mapCatalogSlice(data.profiles, catalogProfileView),
		Templates:             mapCatalogSlice(data.proxmoxTemplates, catalogTemplateView),
		CloudInitTemplates:    catalogCloudInitTemplateDTOs(data.templates, cloudInitWriteEnabled),
		CloudInitWriteEnabled: cloudInitWriteEnabled,
		CloudInitBaselineID:   store.BaselineTemplateID,
		Tags:                  catalogTagDTOs(data.tags),
	}
}

// mapCatalogSlice converts each item with viewOf - the catalog DTO mappers
// below are all this same loop.
func mapCatalogSlice[In, Out any](items []In, viewOf func(In) Out) []Out {
	out := make([]Out, 0, len(items))
	for _, item := range items {
		out = append(out, viewOf(item))
	}

	return out
}

// catalogNodeNames keeps only approved nodes the cluster still reports.
func catalogNodeNames(nodes []catalog.Node, snap cluster.Snapshot) []string {
	discovered := make(map[string]bool, len(snap.Nodes))
	for _, n := range snap.Nodes {
		discovered[n.Name] = true
	}

	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if !discovered[node.Name] {
			continue
		}

		names = append(names, node.Name)
	}

	return names
}

// catalogTagDTOs maps tags, dropping admin-protected ones.
func catalogTagDTOs(tags []catalog.TagWithCount) []catalogTagDTO {
	out := make([]catalogTagDTO, 0, len(tags))
	for _, tag := range tags {
		if tag.Protected {
			continue
		}

		out = append(out, catalogTagDTO{Name: tag.Name, Color: tag.Color})
	}

	return out
}

// catalogStorageDTOs keeps only approved storages that are still VM-capable
// on the live cluster.
func catalogStorageDTOs(storages []catalog.Storage, available []cluster.Storage) []catalogStorageDTO {
	out := make([]catalogStorageDTO, 0, len(storages))
	for _, storage := range storages {
		if _, ok := vmCapableStorage(storage, available); !ok {
			continue
		}

		out = append(out, catalogStorageDTO{Name: storage.Name, Node: storage.Node})
	}

	return out
}

// catalogFileDTOs maps approved file resources (ISOs, cloud images) to DTOs,
// dropping any the cluster no longer reports (the live discovery key set).
func catalogFileDTOs[R, DTO any](resources []R, keyOf func(R) isoDiscoveryKey, live map[isoDiscoveryKey]bool, viewOf func(R) DTO) []DTO {
	out := make([]DTO, 0, len(resources))
	for _, resource := range resources {
		if !live[keyOf(resource)] {
			continue
		}

		out = append(out, viewOf(resource))
	}

	return out
}

// catalogISOKey is the discovery key of an approved ISO row.
func catalogISOKey(iso catalog.ISO) isoDiscoveryKey {
	return isoDiscoveryKey{Node: iso.Node, Storage: iso.Storage, File: iso.File}
}

// catalogImageKey is the discovery key of an approved cloud image row.
func catalogImageKey(image catalog.Image) isoDiscoveryKey {
	return isoDiscoveryKey{Node: image.Node, Storage: image.Storage, File: image.File}
}

// liveISOKeys builds the discovery lookup set for ISOs.
func liveISOKeys(isos []cluster.ISOImage) map[isoDiscoveryKey]bool {
	live := make(map[isoDiscoveryKey]bool, len(isos))
	for _, iso := range isos {
		live[isoDiscoveryKey{Node: iso.Node, Storage: iso.Storage, File: iso.File}] = true
	}

	return live
}

// liveImageKeys builds the discovery lookup set for cloud images.
func liveImageKeys(images []catalog.Image) map[isoDiscoveryKey]bool {
	live := make(map[isoDiscoveryKey]bool, len(images))
	for _, image := range images {
		live[catalogImageKey(image)] = true
	}

	return live
}

// catalogISOView maps an approved ISO row to its catalog DTO.
func catalogISOView(iso catalog.ISO) catalogISODTO {
	return catalogISODTO{Storage: iso.Storage, Node: iso.Node, File: iso.File}
}

// catalogImageView maps an approved cloud image row to its catalog DTO.
func catalogImageView(image catalog.Image) catalogImageDTO {
	return catalogImageDTO{
		Storage: image.Storage, Node: image.Node, File: image.File, SizeBytes: image.SizeBytes,
	}
}

// catalogImageDTOs maps approved cloud images, dropping any the cluster no
// longer reports (the live discovery key set).
func catalogImageDTOs(data catalogData) []catalogImageDTO {
	return catalogFileDTOs(data.resources.Images, catalogImageKey, liveImageKeys(data.images), catalogImageView)
}

// catalogProfileView maps a profile row to its catalog DTO.
func catalogProfileView(profile catalog.Profile) catalogProfileDTO {
	return catalogProfileDTO{
		ID:       profile.ID,
		Label:    profile.Label,
		Sockets:  profile.Sockets,
		CPUCores: profile.CPUCores,
		MemoryMB: profile.MemoryMB,
		DiskGB:   profile.DiskGB,
		Bus:      profile.Bus,
	}
}

// catalogTemplateView maps an approved Proxmox template (clone source) to its catalog DTO.
func catalogTemplateView(tmpl catalog.Template) catalogTemplateDTO {
	return catalogTemplateDTO{
		VMID:             tmpl.VMID,
		Node:             tmpl.Node,
		Name:             tmpl.Name,
		CloudInitCapable: tmpl.CloudInitCapable,
		DiskSizeGB:       tmpl.DiskSizeGB,
		DiskStorage:      tmpl.DiskStorage,
	}
}

// catalogCloudInitTemplateDTOs maps cloud-init templates - the catalog
// exposes only id+label per spec/contracts, never content. The list is empty
// when the cluster has no snippet write target: offering a document the
// create could never write would fail at submit time anyway.
func catalogCloudInitTemplateDTOs(templates []catalog.CloudInitTemplate, writeEnabled bool) []catalogCloudInitTemplateDTO {
	out := make([]catalogCloudInitTemplateDTO, 0, len(templates))
	if !writeEnabled {
		return out
	}

	for _, tmpl := range templates {
		out = append(out, catalogCloudInitTemplateDTO{ID: tmpl.ID, Label: tmpl.Label})
	}

	return out
}

// ServeCatalog handles GET /api/v1/vm-create/catalog. The catalog is the same
// for every user of a cluster (contracts behavioural rules) - no
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

	// The document picker is offered only when this cluster publishes
	// cloud-init documents.
	writeEnabled := false
	if publisher, ok := client.(cluster.SnippetPublisher); ok {
		writeEnabled = publisher.PublishingEnabled()
	}

	dto := buildCatalogDTO(clusterName, data, writeEnabled)

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
// (client bounds are a convenience only).
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
			continue // no capacité configured for this node - nothing to show
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
// SnippetStorageFinder - without this, VM creation ran through the default
// cluster's client regardless of which cluster the request named. The
// HardwareUpdater is needed for post-clone configuration; the
// SnippetStorageFinder for the plan-time snippet storage resolution.
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

	// Optional capability: the clone-time freshness backstop. A client
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

// isoDiscoveryKey is a composite map key for ISOs, avoiding string-concat
// collisions when a storage or file contains ":".
type isoDiscoveryKey struct {
	Node    string
	Storage string
	File    string
}

// catalogBridgeDTOs dedupes by (name, node) - the same bridge name can be
// approved on more than one node, and each is a distinct, independently
// selectable option (bridge approval is per-node, like storage). live carries
// the cluster's current network config, which is where the description
// (Proxmox "comments" field) actually lives - catalog_bridges only stores
// the approval, not the comment. Bridges absent from live (orphan approvals
// whose bridge Proxmox no longer reports) are dropped so users never see a
// bridge they cannot actually use.
func catalogBridgeDTOs(bridges []catalog.Bridge, live []cluster.Bridge) []catalogBridgeDTO {
	type key struct{ name, node string }

	commentByKey := make(map[key]string, len(live))

	liveByKey := make(map[key]bool, len(live))
	for _, bridge := range live {
		commentByKey[key{bridge.Name, bridge.Node}] = bridge.Comment
		liveByKey[key{bridge.Name, bridge.Node}] = true
	}

	out := make([]catalogBridgeDTO, 0, len(bridges))
	seen := make(map[key]struct{}, len(bridges))

	for _, bridge := range bridges {
		k := key{bridge.Name, bridge.Node}
		if _, exists := seen[k]; exists {
			continue
		}

		if !liveByKey[k] {
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
	{vm.ErrDiskBelowImage, http.StatusBadRequest, "disk_below_image", ""},
	{vm.ErrInsufficientDiskSpace, http.StatusBadRequest, "insufficient_disk_space", ""},
	{vm.ErrCloudInitWriteUnavailable, http.StatusConflict, "cloudinit_write_unavailable", "cloud-init documents are not enabled on this cluster (Admin > Clusters: snippet storage and SSH publishing)"},
	{vm.ErrCloudInitNotPublished, http.StatusConflict, "cloudinit_not_published", ""},
	{vm.ErrNoSnippetStorage, http.StatusBadRequest, "no_snippet_storage", ""},
	// cluster_error passes the full error chain (empty message → err.Error()):
	// the Proxmox rejection text ("'import-from' requires special syntax", "has wrong type 'iso'",
	// ...) is the only way to diagnose a 502 from the
	// browser, and the frontend surfaces it after the localized prefix.
	{vm.ErrClusterCreate, http.StatusBadGateway, "cluster_error", ""},
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
