package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"slices"
)

// catalogData is the raw catalog payload loaded from the store and the live
// cluster before it is shaped into the response DTO.
type catalogData struct {
	resources        catalog.Resources
	snap             cluster.Snapshot
	bridges          []cluster.Bridge
	isos             []cluster.ISOImage
	profiles         []catalog.Profile
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

	//
	// for _, image := range images {
	// 	data.images = append(data.images, catalog.Image{Storage: image.Storage, Node: image.Node, File: image.File, SizeBytes: image.SizeBytes})
	// }

	return nil
}

// buildCatalogDTO maps the raw catalog data into the response contract.
// Approved resources are filtered against live discovery: a node, storage,
// bridge, or ISO that Proxmox no longer reports is dropped from the user-facing
// catalog even if its approval row still exists in the store (the admin list
// surfaces those orphans for manual cleanup, and enabled orphans are
// auto-removed there).
func buildCatalogDTO(clusterName string, data catalogData) catalogDTO {
	return catalogDTO{
		Cluster:   clusterName,
		Nodes:     catalogNodeNames(data.resources, data.snap),
		Storages:  catalogStorageDTOs(data.resources.Storages, data.snap.Storages),
		Bridges:   catalogBridgeDTOs(data.resources.Bridges, data.bridges),
		ISOs:      catalogFileDTOs(data.resources.ISOs, catalogISOKey, liveISOKeys(data.isos), catalogISOView),
		Profiles:  mapCatalogSlice(data.profiles, catalogProfileView),
		Templates: mapCatalogSlice(data.proxmoxTemplates, catalogTemplateView),
		Tags:      catalogTagDTOs(data.tags),
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

// catalogNodeNames keeps only approved nodes the cluster still reports and
// that can host a VM: at least one approved storage and bridge on the node.
// Create validates again server-side; this only spares the user a dead end.
func catalogNodeNames(resources catalog.Resources, snap cluster.Snapshot) []string {
	discovered := make(map[string]bool, len(snap.Nodes))
	for _, n := range snap.Nodes {
		discovered[n.Name] = true
	}

	names := make([]string, 0, len(resources.Nodes))
	for _, node := range resources.Nodes {
		onNode := func(n string) bool { return n == node.Name }
		usable := slices.ContainsFunc(resources.Storages, func(s catalog.Storage) bool { return onNode(s.Node) }) &&
			slices.ContainsFunc(resources.Bridges, func(b catalog.Bridge) bool { return onNode(b.Node) })

		if !discovered[node.Name] || !usable {
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

// liveISOKeys builds the discovery lookup set for ISOs.
func liveISOKeys(isos []cluster.ISOImage) map[isoDiscoveryKey]bool {
	live := make(map[isoDiscoveryKey]bool, len(isos))
	for _, iso := range isos {
		live[isoDiscoveryKey{Node: iso.Node, Storage: iso.Storage, File: iso.File}] = true
	}

	return live
}

// catalogISOView maps an approved ISO row to its catalog DTO.
func catalogISOView(iso catalog.ISO) catalogISODTO {
	return catalogISODTO{Storage: iso.Storage, Node: iso.Node, File: iso.File}
}

// catalogImageDTOs maps approved cloud images, dropping any the cluster no
// longer reports (the live discovery key set).
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
		VMID:        tmpl.VMID,
		Node:        tmpl.Node,
		Name:        tmpl.Name,
		DiskSizeGB:  tmpl.DiskSizeGB,
		DiskStorage: tmpl.DiskStorage,
	}
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

	clusterName, client, ok := h.resolveCatalogClient(w, r, identity)
	if !ok {
		return
	}

	data, err := h.loadCatalogData(r.Context(), client, clusterName)
	if err != nil {
		SetErrorMsg(w, "catalog data load failed", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := buildCatalogDTO(clusterName, data)

	if err := h.attachLimits(r.Context(), &dto, clusterName, identity); err != nil {
		SetErrorMsg(w, "policy read failed", err)
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

func (h *VMCreate) resolveCatalogClient(w http.ResponseWriter, r *http.Request, identity auth.Identity) (string, cluster.Client, bool) {
	requested, allowed := vm.TargetCluster(identity, r.URL.Query().Get("cluster"))
	if !allowed {
		h.writeCreateError(w, http.StatusNotFound, "not_found", msgClusterNotFound)

		return "", nil, false
	}

	clusterName, err := ResolveClusterValue(requested, h.clients)
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
