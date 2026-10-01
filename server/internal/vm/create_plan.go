package vm

import (
	"context"
	"fmt"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

// createPlan holds the resolved and validated values for a VM creation request.
type createPlan struct {
	node    string
	storage string
	// document is the published cloud-init file resolved at plan time and
	// proven present on the node: the requested template, or the baseline
	// for an image VM without one. Empty on the plain ISO path.
	document publishedDocument
	// documentSkipReason explains an empty document on the image path so
	// the baseline state reports the real cause.
	documentSkipReason string
	sockets            int
	cpuCores           int
	memoryMB           int
	diskGB             int
	bus                string
	// imageSizeGB is the cloud image's size in whole GB (rounded up), set
	// only in image mode. import-from lands the disk at this size; the
	// caller grows it to diskGB after the create task completes.
	imageSizeGB      int
	nics             []nicPlan
	isolationVLANTag int
	uefi             bool
	tpm              bool
}

// nicPlan is one resolved and validated NIC.
type nicPlan struct {
	bridge string
	model  string
}

// checkName validates the hostname form then checks per-pool name
// uniqueness. A malformed name reports ErrInvalidName
// before the duplicate check runs. Extracted from planCreate to keep its
// cyclomatic complexity under gocyclo's ceiling.
func checkName(policyService *policy.Policy, pool, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	// per-pool name uniqueness. The name is the only
	// identifier the user manipulates in the portal; two VMs with the same
	// name in one user's list are indistinguishable. Checked before any
	// VMID is consumed, like every other rejection.
	if policyService.PoolHasName(pool, name) {
		return fmt.Errorf("%w: %q is already used by a VM in your pool", ErrNameTaken, name)
	}

	return nil
}

// resolveUEFI defaults UEFI to true when the request omits it (modern OSes expect UEFI boot).
// An explicit false selects legacy SeaBIOS.
//
// Image mode is the exception: a cloud image
// ships a stripped-down kernel (Debian's linux-image-cloud-amd64 has
// # CONFIG_DRM is not set) that cannot drive the emulated VGA under UEFI, so
// the graphical console renders as static. Defaulting image mode to SeaBIOS
// makes the graphical tab show the guest's real text console from first boot.
// The wizard's UEFI checkbox stays visible and re-tickable; a request that
// sends UEFI=true explicitly still creates a UEFI VM (TPM/Secure Boot keep
// their "requires UEFI" behaviour via checkUEFICompat).
func resolveUEFI(req CreateRequest) bool {
	if req.UEFI != nil {
		return *req.UEFI
	}

	if req.Image != nil {
		return false
	}

	return true
}

// checkUEFICompat rejects the impossible TPM-without-UEFI combination early
// (TPM 2.0 requires UEFI). Extracted from planCreate to keep its cyclomatic
// complexity under gocyclo's ceiling.
func checkUEFICompat(req CreateRequest) error {
	if !resolveUEFI(req) && req.TPM {
		return fmt.Errorf("%w: tpm requires uefi", ErrInvalidRequest)
	}

	return nil
}

// planCreate runs all pre-allocation validation: quota, name, catalog,
// hardware ranges, gabarit, resource resolution, node capacity, and live
// disk-space check. Name uniqueness by pool
// is checked after ValidateName so a malformed name reports ErrInvalidName
// before the duplicate check runs.
func planCreate(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (createPlan, error) {
	if err := policyService.CheckQuota(ctx, clusterName, actor); err != nil {
		return createPlan{}, err
	}

	if err := checkName(policyService, actor.Pool, req.Name); err != nil {
		return createPlan{}, err
	}

	// TPM 2.0 requires UEFI - reject the impossible
	// combination early, before any VMID or catalog work.
	if err := checkUEFICompat(req); err != nil {
		return createPlan{}, err
	}

	resources, err := catalog.ApprovedResources(ctx, deps.Store, clusterName)
	if err != nil {
		return createPlan{}, fmt.Errorf("read catalog: %w", err)
	}

	sockets, cpuCores, memoryMB, diskGB, bus, err := resolveHardware(ctx, deps.Store, clusterName, req)
	if err != nil {
		return createPlan{}, err
	}

	if err := checkTechnicalRange(cpuCores, memoryMB, diskGB); err != nil {
		return createPlan{}, err
	}

	// Fetch node capacities for placement scoring and storage
	// free space from the projection for best-storage selection, then resolve
	// and validate the placement (node/storage/NICs against the catalog).
	node, storage, nics, err := resolvePlacement(ctx, req, policyService, clusterName, resources, deps.Log)
	if err != nil {
		return createPlan{}, err
	}

	// (image mode): reject a disk size below the cloud image before any
	// VMID is spent - the import lands at the image's size and only grows.
	// Run after planCreate so the check sees the resolved disk size (profile
	// overrides applied). The image size rides on the plan so createFromImage
	// can grow the imported disk to the requested size.
	var imageSizeGB int

	if req.Image != nil {
		imageSizeGB, err = checkDiskAboveImage(resources, req, node, diskGB)
		if err != nil {
			return createPlan{}, err
		}
	}

	if err := checkGabaritAndCapacity(ctx, policyService, gabaritRequest{
		clusterName: clusterName, node: node,
		sockets: sockets, cpuCores: cpuCores, memoryMB: memoryMB, diskGB: diskGB,
		nicCount: len(nics),
	}); err != nil {
		return createPlan{}, err
	}

	vlanTag, err := finalizePlanChecks(ctx, deps, policyService, clusterName, node, storage, diskGB)
	if err != nil {
		return createPlan{}, err
	}

	document, skipReason, err := resolvePlanDocument(ctx, deps, documentTarget{Cluster: clusterName, Node: node}, req)
	if err != nil {
		return createPlan{}, err
	}

	return createPlan{
		node: node, storage: storage, document: document, documentSkipReason: skipReason,
		sockets: sockets, cpuCores: cpuCores,
		memoryMB: memoryMB, diskGB: diskGB, bus: bus, nics: nics,
		isolationVLANTag: vlanTag, uefi: resolveUEFI(req), tpm: req.TPM,
		imageSizeGB: imageSizeGB,
	}, nil
}

// resolvePlacement fetches node capacities and storage free space from the
// projection, resolves node/storage/NICs, validates the choice
// against the catalog, and logs the placement decision when auto-selection
// ran. Extracted from planCreate to keep its cyclomatic complexity
// under gocyclo's ceiling.
func resolvePlacement(ctx context.Context, req CreateRequest, policyService *policy.Policy, clusterName string, resources catalog.Resources, log *slog.Logger) (node, storage string, nics []nicPlan, err error) {
	capacities := fetchNodeCapacities(ctx, policyService, clusterName, resources.Nodes)
	storageFree := fetchStorageFreeBytes(policyService, resources.Storages)

	node, storage, nics, err = resolveResources(req, resources, capacities, storageFree)
	if err != nil {
		return "", "", nil, err
	}

	if err := validateCatalog(req, resources, node, storage, nics); err != nil {
		return "", "", nil, err
	}

	if req.Node == "" && log != nil {
		logPlacement(log, node, resources.Nodes, capacities, req)
	}

	return node, storage, nics, nil
}

// gabaritRequest groups the resolved hardware dimensions a gabarit + capacity
// check needs. Extracted from checkGabaritAndCapacity's parameter list to
// stay under go:S107's 7-parameter ceiling.
type gabaritRequest struct {
	clusterName string
	node        string
	sockets     int
	cpuCores    int
	memoryMB    int
	diskGB      int
	nicCount    int
}

// checkGabaritAndCapacity runs the gabarit ceiling check and the node-capacity
// check together. Extracted from planCreate to keep
// its cyclomatic complexity under gocyclo's ceiling.
func checkGabaritAndCapacity(ctx context.Context, policyService *policy.Policy, req gabaritRequest) error {
	if err := policyService.CheckGabarit(ctx, req.clusterName, req.sockets, req.cpuCores, req.memoryMB, req.diskGB, req.nicCount); err != nil {
		return err
	}

	return policyService.CheckNodeCapacity(ctx, req.clusterName, req.node, policy.CapacityDelta{
		Sockets: req.sockets, Cores: req.cpuCores, MemoryMB: req.memoryMB, DiskGB: req.diskGB,
	})
}

// finalizePlanChecks runs the post-capacity checks: the live disk-space
// check and the gabarit VLAN read.
// Returns the per-cluster isolation VLAN tag (0 = none imposed). Extracted
// from planCreate to keep its cyclomatic complexity under gocyclo's ceiling.
func finalizePlanChecks(ctx context.Context, deps CreateDeps, policyService *policy.Policy, clusterName, node, storage string, diskGB int) (int, error) {
	if err := checkLiveDiskSpace(ctx, deps.FreeSpace, node, storage, diskGB); err != nil {
		return 0, err
	}

	gabarit, err := policyService.Gabarit(ctx, clusterName)
	if err != nil {
		return 0, fmt.Errorf("read gabarit for vlan: %w", err)
	}

	return gabarit.IsolationVLANTag, nil
}

// checkLiveDiskSpace verifies the target storage has enough free space for the
// requested disk. Skipped when no FreeSpaceChecker is wired
// (unit tests that don't need the live check) or when diskGB is zero.
func checkLiveDiskSpace(ctx context.Context, freeSpace FreeSpaceChecker, node, storage string, diskGB int) error {
	if freeSpace == nil || diskGB <= 0 {
		return nil
	}

	freeBytes, err := freeSpace.StorageFreeSpace(ctx, node, storage)
	if err != nil {
		return fmt.Errorf("%w: read free space on %q/%q: %w", ErrClusterCreate, node, storage, err)
	}

	needed := int64(diskGB) * bytesPerGB
	if freeBytes < needed {
		return fmt.Errorf("%w: storage %q on node %q has %d GB free, request needs %d GB", ErrInsufficientDiskSpace, storage, node, freeBytes/bytesPerGB, int64(diskGB))
	}

	return nil
}

// bytesPerGB is the conversion factor for disk-space checks.
const bytesPerGB int64 = 1024 * 1024 * 1024

// fetchNodeCapacities reads the capacity of each approved node from the policy
// service. Nodes with no configured capacité return a zero-value Capacity
// (scoreNode handles this gracefully).
func fetchNodeCapacities(ctx context.Context, policyService *policy.Policy, clusterName string, nodes []catalog.Node) map[string]policy.Capacity {
	capacities := make(map[string]policy.Capacity, len(nodes))

	for _, n := range nodes {
		nodeCap, err := policyService.NodeCapacity(ctx, clusterName, n.Name)
		if err != nil {
			continue
		}

		capacities[n.Name] = nodeCap
	}

	return capacities
}

// fetchStorageFreeBytes reads the projected free bytes for each approved
// storage from the policy service's in-memory projection. Storages not in the
// projection get 0 (bestStorageOnNode treats 0 as a valid candidate).
func fetchStorageFreeBytes(policyService *policy.Policy, storages []catalog.Storage) map[string]int64 {
	free := make(map[string]int64, len(storages))

	for _, s := range storages {
		free[s.Name] = policyService.StorageFreeBytes(s.Node, s.Name)
	}

	return free
}

// logPlacement emits a [placement] log line naming each candidate and its
// score, like ProxMate's scheduler log.
func logPlacement(log *slog.Logger, selected string, candidates []catalog.Node, capacities map[string]policy.Capacity, req CreateRequest) {
	scores := make([]string, 0, len(candidates))

	for _, n := range candidates {
		score := scoreNode(capacities[n.Name], req)
		scores = append(scores, fmt.Sprintf("%s=%.3f", n.Name, score))
	}

	log.Info("[placement] auto-selected node", "component", "vm", "selected", selected, "candidates", scores)
}

// resolveHardware returns the effective sockets, CPU, memory, disk, and bus
// values, applying the profile's catalog values when a profile is selected.
// Sockets defaults to 1 when the request omits it (zero value).
func resolveHardware(ctx context.Context, st *store.Store, clusterName string, req CreateRequest) (sockets, cpuCores, memoryMB, diskGB int, bus string, err error) {
	sockets, cpuCores, memoryMB, diskGB = defaultSockets(req.Sockets), req.CPUCores, req.MemoryMB, req.Disk.SizeGB
	bus = defaultDiskBus

	if req.ProfileID == "" {
		return sockets, cpuCores, memoryMB, diskGB, bus, nil
	}

	profiles, err := catalog.Profiles(ctx, st, clusterName)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("read profiles: %w", err)
	}

	profile, err := catalog.FindProfile(profiles, req.ProfileID)
	if err != nil {
		return 0, 0, 0, 0, "", notApprovedError(err)
	}

	// The profile's catalog values are authoritative - hardware
	// fields the request also carries are ignored, never merged.
	return profile.Sockets, profile.CPUCores, profile.MemoryMB, profile.DiskGB, profile.Bus, nil
}

// defaultSockets returns n or 1 when n is zero - the Proxmox default and the
// value every existing request implicitly used.
func defaultSockets(n int) int {
	if n == 0 {
		return 1
	}

	return n
}

// Placement scoring weights (fixed, matching ProxMate).
// Revisit only if a real deployment demonstrates bad placement.
const (
	placementWeightMem  = 0.5
	placementWeightCPU  = 0.35
	placementWeightDisk = 0.15
	placementFitBonus   = 1.0
)

// resolveResources resolves the node, storage, and NICs, applying
// auto-selection defaults when the request omits them. When the request
// carries an ISO and no explicit node, candidate nodes are restricted to
// those that hold the ISO (a node-local ISO silently fails on the wrong node - the refusal must
// arrive before VMID consumption).
//
// When no explicit node is selected, candidates are scored by free resource
// fractions: memFrac*0.5 + cpuFrac*0.35 + diskFrac*0.15, +1
// if the VM fits. Catalog order breaks ties for reproducibility.
func resolveResources(req CreateRequest, resources catalog.Resources, capacities map[string]policy.Capacity, storageFree map[string]int64) (node, storage string, nics []nicPlan, err error) {
	node, err = resolveNode(req, resources, capacities)
	if err != nil {
		return "", "", nil, err
	}

	storage = req.Disk.Storage
	if storage == "" {
		storage = bestStorageOnNode(resources, node, storageFree)
		if storage == "" {
			return "", "", nil, fmt.Errorf("%w: no approved storage on node %q", ErrNotApproved, node)
		}
	}

	nics, err = resolveNICs(req, resources, node)
	if err != nil {
		return "", "", nil, err
	}

	return node, storage, nics, nil
}

// resolveNode returns the requested node, or - when the request omits it -
// the best-scoring approved node. Candidates are restricted to nodes holding
// the requested ISO or image, then hard-filtered to nodes with at least one
// approved storage.
func resolveNode(req CreateRequest, resources catalog.Resources, capacities map[string]policy.Capacity) (string, error) {
	if req.Node != "" {
		return req.Node, nil
	}

	candidates := resources.Nodes
	if req.ISO != nil {
		candidates = nodesWithISO(resources, req.ISO.Storage, req.ISO.File)
		if len(candidates) == 0 {
			return "", fmt.Errorf("%w: no approved node holds iso %q on storage %q", ErrNotApproved, req.ISO.File, req.ISO.Storage)
		}
	}

	if req.Image != nil {
		candidates = nodesWithImage(resources, req.Image.Storage, req.Image.File)
		if len(candidates) == 0 {
			return "", fmt.Errorf("%w: no approved node holds image %q on storage %q", ErrNotApproved, req.Image.File, req.Image.Storage)
		}
	}

	// Hard filter: node must have at least one approved storage.
	candidates = nodesWithStorage(resources, candidates)
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: no approved node with storage in catalog", ErrNotApproved)
	}

	return pickBestNode(candidates, capacities, req), nil
}

// pickBestNode scores each candidate node and returns the name of the highest
// scorer. Catalog order breaks ties (stable selection for reproducible tests).
func pickBestNode(candidates []catalog.Node, capacities map[string]policy.Capacity, req CreateRequest) string {
	best := candidates[0]
	bestScore := scoreNode(capacities[best.Name], req)

	for _, candidate := range candidates[1:] {
		score := scoreNode(capacities[candidate.Name], req)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	return best.Name
}

// scoreNode computes a placement score from free resource fractions.
// The formula matches ProxMate's fixed weights:
// memFrac*0.5 + cpuFrac*0.35 + diskFrac*0.15, +1 if the VM fits. A node with
// no capacity data (zero value) scores 0 - still selectable as a fallback,
// but preferred less than any node with known headroom. A configured cap
// narrower than physical capacity becomes the effective ceiling, so capping
// a node steers placement away before the hard capacity check ever fires.
func scoreNode(capacity policy.Capacity, req CreateRequest) float64 {
	ramCeiling := effectiveCeiling(capacity.MaxRAMGB, capacity.PhysicalRAMGB)
	cpuCeiling := effectiveCeiling(capacity.MaxVCPUs, capacity.PhysicalVCPUs)

	score := freeFraction(capacity.UsedRAMGB, ramCeiling)*placementWeightMem +
		freeFraction(capacity.UsedVCPUs, cpuCeiling)*placementWeightCPU +
		freeFraction(capacity.UsedDiskGB, capacity.MaxDiskGB)*placementWeightDisk

	// Bonus if the VM actually fits (bonus, not barrier - under overcommit
	// a node is still returned rather than failing the create).
	requestedRAM := (req.MemoryMB + 1023) / 1024
	requestedCPU := defaultSockets(req.Sockets) * req.CPUCores

	fitsMem := ramCeiling == 0 || ramCeiling-capacity.UsedRAMGB >= requestedRAM
	fitsCPU := cpuCeiling == 0 || cpuCeiling-capacity.UsedVCPUs >= requestedCPU
	fitsDisk := capacity.MaxDiskGB == 0 || capacity.MaxDiskGB-capacity.UsedDiskGB >= req.Disk.SizeGB
	fitsVMs := capacity.MaxVMs == 0 || capacity.UsedVMs < capacity.MaxVMs

	if fitsMem && fitsCPU && fitsDisk && fitsVMs {
		score += placementFitBonus
	}

	return score
}

// freeFraction is the free share of a ceiling (0 when the ceiling is unknown
// or already exhausted).
func freeFraction(used, ceiling int) float64 {
	if ceiling <= 0 {
		return 0
	}

	free := ceiling - used
	if free <= 0 {
		return 0
	}

	return float64(free) / float64(ceiling)
}

// effectiveCeiling returns the tighter of a configured cap and the physical
// capacity; either bound may be absent (0 = unbounded).
func effectiveCeiling(configured, physical int) int {
	switch {
	case configured <= 0:
		return physical
	case physical <= 0 || configured < physical:
		return configured
	default:
		return physical
	}
}

// nodesWithStorage filters candidates to those that have at least one approved
// storage in the catalog (hard filter).
func nodesWithStorage(resources catalog.Resources, candidates []catalog.Node) []catalog.Node {
	var filtered []catalog.Node

	for _, candidate := range candidates {
		for _, storage := range resources.Storages {
			if storage.Node == candidate.Name {
				filtered = append(filtered, candidate)
				break
			}
		}
	}

	return filtered
}

// resolveNICs builds the resolved NIC list from the request. An empty request
// list produces one auto-selected NIC (simple mode); each entry with an empty
// bridge gets the first approved bridge on the node, and each entry with an
// empty model gets the default model.
func resolveNICs(req CreateRequest, resources catalog.Resources, node string) ([]nicPlan, error) {
	requested := req.Network
	if len(requested) == 0 {
		bridge := firstBridgeOnNode(resources, node)
		if bridge == "" {
			return nil, fmt.Errorf("%w: no approved bridge on node %q", ErrNotApproved, node)
		}

		return []nicPlan{{bridge: bridge, model: defaultNetworkModel}}, nil
	}

	nics := make([]nicPlan, 0, len(requested))
	for _, reqNIC := range requested {
		bridge := reqNIC.Bridge
		if bridge == "" {
			bridge = firstBridgeOnNode(resources, node)
			if bridge == "" {
				return nil, fmt.Errorf("%w: no approved bridge on node %q", ErrNotApproved, node)
			}
		}

		model := reqNIC.Model
		if model == "" {
			model = defaultNetworkModel
		}

		nics = append(nics, nicPlan{bridge: bridge, model: model})
	}

	return nics, nil
}

// nodesWithISO returns the approved nodes that hold the given ISO, preserving
// catalog order so auto-selection is deterministic.
func nodesWithISO(resources catalog.Resources, storage, file string) []catalog.Node {
	var matched []catalog.Node

	for _, node := range resources.Nodes {
		if resources.HasISO(storage, file, node.Name) {
			matched = append(matched, node)
		}
	}

	return matched
}

// nodesWithImage returns the approved nodes that hold the given cloud image,
// preserving catalog order so auto-selection is deterministic.
func nodesWithImage(resources catalog.Resources, storage, file string) []catalog.Node {
	var matched []catalog.Node

	for _, node := range resources.Nodes {
		if resources.HasCloudImage(storage, file, node.Name) {
			matched = append(matched, node)
		}
	}

	return matched
}

// validateCatalog checks that the resolved node, storage, each NIC's bridge
// and model, the optional ISO, and every requested tag are all present in the
// approved catalog.
func validateCatalog(req CreateRequest, resources catalog.Resources, node, storage string, nics []nicPlan) error {
	if !resources.HasNode(node) {
		return fmt.Errorf("%w: node %q", ErrNotApproved, node)
	}

	if !resources.HasStorage(storage, node) {
		return fmt.Errorf("%w: storage %q on node %q", ErrNotApproved, storage, node)
	}

	for _, nic := range nics {
		if !allowedNetworkModels[nic.model] {
			return fmt.Errorf("%w: network model %q", ErrNotApproved, nic.model)
		}

		if !resources.HasBridge(nic.bridge, node) {
			return fmt.Errorf("%w: bridge %q on node %q", ErrNotApproved, nic.bridge, node)
		}
	}

	if req.ISO != nil && !resources.HasISO(req.ISO.Storage, req.ISO.File, node) {
		return fmt.Errorf("%w: iso %q on storage %q on node %q", ErrNotApproved, req.ISO.File, req.ISO.Storage, node)
	}

	if req.Image != nil && !resources.HasCloudImage(req.Image.Storage, req.Image.File, node) {
		return fmt.Errorf("%w: image %q on storage %q on node %q", ErrNotApproved, req.Image.File, req.Image.Storage, node)
	}

	for _, tag := range req.Tags {
		if !resources.HasTag(tag) {
			return fmt.Errorf("%w: tag %q", ErrNotApproved, tag)
		}
	}

	return nil
}

// checkTechnicalRange enforces the fixed anti-abuse bounds.
func checkTechnicalRange(cpuCores, memoryMB, diskGB int) error {
	switch {
	case cpuCores < MinCPUCores || cpuCores > MaxCPUCores:
		return fmt.Errorf("%w: cpuCores must be between %d and %d", ErrOutOfRange, MinCPUCores, MaxCPUCores)
	case memoryMB < MinMemoryMB || memoryMB > MaxMemoryMB:
		return fmt.Errorf("%w: memoryMB must be between %d and %d", ErrOutOfRange, MinMemoryMB, MaxMemoryMB)
	case diskGB < MinDiskGB || diskGB > MaxDiskGB:
		return fmt.Errorf("%w: disk sizeGB must be between %d and %d", ErrOutOfRange, MinDiskGB, MaxDiskGB)
	}

	return nil
}

// firstStorageOnNode returns the first approved storage attached to node, or "" when none is
// (catalog queries are ordered, so this is deterministic).
// bestStorageOnNode picks the approved storage with the most free space on
// the selected node. Falls back to catalog order when
// free-space data is unavailable (zero free bytes). Catalog order breaks ties
// for reproducibility.
func bestStorageOnNode(resources catalog.Resources, node string, storageFree map[string]int64) string {
	best := ""
	bestFree := int64(-1)

	for _, storage := range resources.Storages {
		if storage.Node != node {
			continue
		}

		free := storageFree[storage.Name]
		if free > bestFree {
			best = storage.Name
			bestFree = free
		}
	}

	return best
}

func firstBridgeOnNode(resources catalog.Resources, node string) string {
	for _, bridge := range resources.Bridges {
		if bridge.Node == node {
			return bridge.Name
		}
	}

	return ""
}
