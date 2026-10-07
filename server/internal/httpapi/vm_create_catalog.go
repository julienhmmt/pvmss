package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"slices"
)

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
	templateNodes    map[string][]string
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

	data.templates, data.templateNodes = templatesOnNodes(ctx, client, templates)

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
		Nodes:                 catalogNodeNames(data.resources, data.snap),
		Storages:              catalogStorageDTOs(data.resources.Storages, data.snap.Storages),
		Bridges:               catalogBridgeDTOs(data.resources.Bridges, data.bridges),
		ISOs:                  catalogFileDTOs(data.resources.ISOs, catalogISOKey, liveISOKeys(data.isos), catalogISOView),
		Images:                catalogImageDTOs(data),
		Profiles:              mapCatalogSlice(data.profiles, catalogProfileView),
		Templates:             mapCatalogSlice(data.proxmoxTemplates, catalogTemplateView),
		CloudInitTemplates:    catalogCloudInitTemplateDTOs(data.templates, data.templateNodes, cloudInitWriteEnabled),
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

// templatesOnNodes keeps the templates whose file is on at least one node,
// with those nodes, read live through the API. A template on no node would
// be refused at create time anyway. A cluster that cannot check offers none.
func templatesOnNodes(ctx context.Context, client cluster.Client, templates []catalog.CloudInitTemplate) ([]catalog.CloudInitTemplate, map[string][]string) {
	checker, ok := client.(cluster.SnippetChecker)
	if !ok || checker.SnippetStorageID() == "" {
		return nil, nil
	}

	var offered []catalog.CloudInitTemplate

	nodes := make(map[string][]string, len(templates))

	for _, t := range templates {
		status, err := catalog.CheckCloudInitDocument(ctx, checker, t.ID, t.Content)
		if err != nil {
			continue
		}

		for _, n := range status.Nodes {
			if n.Present {
				nodes[t.ID] = append(nodes[t.ID], n.Node)
			}
		}

		if len(nodes[t.ID]) > 0 {
			offered = append(offered, t)
		}
	}

	return offered, nodes
}

// catalogCloudInitTemplateDTOs maps cloud-init templates - the catalog
// exposes id, label and the nodes that have the file, never content. The
// list is empty when the cluster has no snippet storage.
func catalogCloudInitTemplateDTOs(templates []catalog.CloudInitTemplate, nodes map[string][]string, writeEnabled bool) []catalogCloudInitTemplateDTO {
	out := make([]catalogCloudInitTemplateDTO, 0, len(templates))
	if !writeEnabled {
		return out
	}

	for _, tmpl := range templates {
		out = append(out, catalogCloudInitTemplateDTO{ID: tmpl.ID, Label: tmpl.Label, Nodes: nodes[tmpl.ID]})
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
		SetErrorMsg(w, "catalog data load failed", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	// The document picker is offered only when this cluster has a snippet
	// storage for cloud-init documents.
	writeEnabled := false
	if checker, ok := client.(cluster.SnippetChecker); ok {
		writeEnabled = checker.SnippetStorageID() != ""
	}

	dto := buildCatalogDTO(clusterName, data, writeEnabled)

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
