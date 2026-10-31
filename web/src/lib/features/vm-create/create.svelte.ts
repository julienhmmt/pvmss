import { getContext, setContext } from 'svelte';
import { get, post, ApiRequestError } from '#lib/shared/api/client.js';
import { fetchClusterOptions, pinnedClusterOptions, type ClusterOption } from '#lib/shared/clusters.js';
import { m } from '#lib/paraglide/messages.js';

export interface CatalogStorage {
	name: string;
	node: string;
}

/** One approved bridge on one node - bridge approval is per-node, like
 *  storage, so a bridge option only makes sense scoped to the VM's node. */
export interface CatalogBridge {
	name: string;
	node: string;
	comment?: string;
}

export interface CatalogISO {
	storage: string;
	node: string;
	file: string;
}

export interface CatalogProfile {
	id: string;
	label: string;
	sockets: number;
	cpuCores: number;
	memoryMB: number;
	diskGB: number;
	bus: string;
}

/** One approved Proxmox template (US2/issue-02). The VMID is the Proxmox
 *  VMID of the template VM; the node determines where the clone lands
 *  (D2b: cross-node clone is forbidden). DiskSizeGB is the template's
 *  disk size (reductions are rejected). DiskStorage is where the template's
 *  disk lives - the clone *source* storage, used to warn when the target
 *  storage differs (full copy instead of linked clone). */
export interface CatalogTemplate {
	vmid: number;
	node: string;
	name: string;
	diskSizeGB: number;
	diskStorage: string;
}

export interface CatalogTag {
	name: string;
	color: string;
}

/** The administrator-editable per-VM size ceiling (gabarit) - the same
 *  bounds the server re-checks on submit (constitution VI). */
export interface CatalogGabarit {
	maxSockets: number;
	maxCores: number;
	maxMemoryMB: number;
	maxDiskPerVMGB: number;
	maxNetworkCards: number;
	maxSnapshots: number;
	isolationVlanTag: number;
}

/** Memory for display: whole gigabytes as GB, anything else in MB. */
// Proxmox memory is MiB, so the units are binary - as everywhere else in the UI.
export function memoryLabel(memoryMB: number): string {
	return memoryMB >= 1024 && memoryMB % 1024 === 0 ? `${memoryMB / 1024} GiB` : `${memoryMB} MiB`;
}

/** The effective UI ceiling for a gabarit field: 0 means no cap (the server
 *  enforces the same rule), so the fallback rail applies for the input's
 *  max attribute. */
export function gabaritCeiling(value: number | undefined, fallback: number): number {
	return value != null && value > 0 ? value : fallback;
}

/** The caller's pool VM count against the cluster's pool allowance.
 *  allowed is -1 for unlimited. */
export interface CatalogQuota {
	used: number;
	allowed: number;
}

/** One approved node's configured aggregate capacité, live usage, and
 *  physical facts. Absent from `nodeCapacities` for a node with no
 *  capacité configured. */
export interface CatalogNodeCapacity {
	node: string;
	maxVMs: number;
	maxVCPUs: number;
	maxRAMGB: number;
	maxDiskGB: number;
	usedVMs: number;
	usedVCPUs: number;
	usedRAMGB: number;
	physicalVCPUs: number;
	physicalRAMGB: number;
}

export interface VmCreateCatalog {
	cluster: string;
	nodes: string[];
	storages: CatalogStorage[];
	bridges: CatalogBridge[];
	isos: CatalogISO[];
	profiles: CatalogProfile[];
	templates: CatalogTemplate[];
	tags: CatalogTag[];
	gabarit?: CatalogGabarit;
	quota?: CatalogQuota;
	nodeCapacities?: CatalogNodeCapacity[];
}

/** One network interface card in the creation request (US2/D3a). */
export interface NICRequest {
	bridge: string;
	model: string;
}

/** One editable NIC row in the form's local state (US2/D3a). The shape
 *  mirrors NICRequest but is a distinct type so the form owns its model. */
export interface NICRow {
	bridge: string;
	model: string;
}

/** The single request shape both modes POST (FR-001) - no pool, no mode.
 *  The VM source is either an ISO (iso field) or a Proxmox template
 *  (templateId field), never both (US2/issue-02 D2a). */
export interface VMCreateRequest {
	cluster: string;
	name: string;
	profileId?: string;
	node?: string;
	tags?: string[];
	sockets?: number;
	cpuCores?: number;
	memoryMB?: number;
	disk?: { storage?: string; sizeGB?: number };
	network?: NICRequest[];
	iso?: { storage: string; file: string };
	templateId?: number;
	uefi?: boolean;
	tpm?: boolean;
	startAfterCreate?: boolean;
}

export interface VmCreateAccepted {
	cluster: string;
	vmid: number;
	name: string;
	node: string;
	upid: string;
	expectedStatus?: 'running' | 'stopped';
}

export type CreateMode = 'simple' | 'detailed';

/** The VM source type (US2/issue-02 D2a): 'iso' for OS without templates
 *  (Windows, appliances) or 'template' for a Proxmox template clone.
 *  The two are mutually exclusive - the server rejects a request carrying
 *  more than one. */
export type VmSource = 'iso' | 'template';

/** Simple-mode source (V08): the user can either pick a size profile or
 *  clone from an approved Proxmox template. This is a UI-only distinction;
 *  the request still carries profileId or templateId. */
export type SimpleSource = 'profile' | 'template';

/** Conversion factor for disk-size checks - mirrors the server's
 *  bytesPerGB (server/internal/vm/create.go). */

/** Server error codes with fixed, non-parameterized text (server/internal/httpapi/vm_create.go). */
const FIXED_SUBMIT_ERRORS: Partial<Record<string, () => string>> = {
	admin_cannot_create: m['vms.create.adminBlocked'],
	no_pool: m['vms.create.errorNoPool'],
	invalid_name: m['vms.create.errorInvalidName'],
	name_taken: m['vms.create.errorNameTaken'],
	invalid_source: m['vms.create.errorInvalidSource'],
	invalid_request: m['vms.create.errorInvalidRequest'],
	disk_reduction: m['vms.create.errorDiskReduction'],
	insufficient_disk_space: m['vms.create.errorInsufficientDiskSpace'],
	cluster_error: m['vms.create.errorClusterRejected'],
	internal_error: m['vms.create.errorInternal']
};

/** Maps a server capacity dimension string ("vms"/"vcpu"/"ram"/"disk") to its
 *  localized label. The server emits "vcpu" (singular) for the vCPUs dimension
 *  (server/internal/policy/policy.go: dimensionVCPU). */
const CAPACITY_DIMENSION_LABELS: Record<string, () => string> = {
	vms: m['vms.create.capacityDimension.vms'],
	vcpu: m['vms.create.capacityDimension.vcpu'],
	ram: m['vms.create.capacityDimension.ram'],
	disk: m['vms.create.capacityDimension.disk']
};

/** Maps a server gabarit field name to its localized label
 *  (server/internal/policy/guards.go: GabaritExceededError.Field). */
const GABARIT_FIELD_LABELS: Record<string, () => string> = {
	sockets: m['vms.create.fieldLabel.sockets'],
	cores: m['vms.create.fieldLabel.cores'],
	memoryMB: m['vms.create.fieldLabel.memoryMB'],
	diskGB: m['vms.create.fieldLabel.diskGB'],
	networkCards: m['vms.create.fieldLabel.networkCards']
};

/** Parses the server's "node %q %s capacity (%d) would be exceeded" message
 *  into a localized string. The dimension word is one of vms/vcpu/ram/disk. */
function translateCapacityExceeded(message: string): string {
	// Message shape: node "miniquarium" disk capacity (48) would be exceeded
	const match = message.match(/^node "([^"]+)" (\w+) capacity \((\d+)\) would be exceeded$/);
	if (!match) return m['vms.create.errorCreation']();
	const node = match[1] ?? '';
	const dimension = match[2] ?? '';
	const max = match[3] ?? '0';
	const dimensionLabel = CAPACITY_DIMENSION_LABELS[dimension]?.() ?? dimension;
	return m['vms.create.errorCapacityExceeded']({ node, dimension: dimensionLabel, max: Number(max) });
}

/** Parses the server's `pool "p" already holds %d of %d allowed VMs` message
 *  into a localized string. */
function translateQuotaExceeded(message: string): string {
	const match = message.match(/already holds (\d+) of (\d+) allowed VMs$/);
	if (!match) return m['vms.create.errorCreation']();
	const used = match[1] ?? '0';
	const allowed = match[2] ?? '0';
	return m['vms.create.errorQuotaExceeded']({ used: Number(used), allowed: Number(allowed) });
}

/** Parses the server's gabarit error messages into a localized string. The
 *  server emits per-field shapes: "disk size (N GB) exceeds the configured
 *  gabarit (M GB)", "memory (N MB) exceeds ...", "network cards (N) exceed
 *  ...", and a generic "%s (N) exceeds the configured gabarit (M)". */
function translateGabaritExceeded(message: string): string {
	const patterns: Array<{ re: RegExp; field: string; requestedGroup: number; maximumGroup: number }> = [
		{ re: /^disk size \((\d+) GB\) exceeds the configured gabarit \((\d+) GB\)$/, field: 'diskGB', requestedGroup: 1, maximumGroup: 2 },
		{ re: /^memory \((\d+) MB\) exceeds the configured gabarit \((\d+) MB\)$/, field: 'memoryMB', requestedGroup: 1, maximumGroup: 2 },
		{ re: /^network cards \((\d+)\) exceed the configured gabarit \((\d+)\)$/, field: 'networkCards', requestedGroup: 1, maximumGroup: 2 },
		{ re: /^(\w+) \((\d+)\) exceeds the configured gabarit \((\d+)\)$/, field: '', requestedGroup: 2, maximumGroup: 3 }
	];
	for (const { re, field, requestedGroup, maximumGroup } of patterns) {
		const match = message.match(re);
		if (!match) continue;
		const fieldName = field || (match[1] ?? '');
		const fieldLabel = GABARIT_FIELD_LABELS[fieldName]?.() ?? fieldName;
		return m['vms.create.errorGabaritExceeded']({
			field: fieldLabel,
			requested: Number(match[requestedGroup] ?? '0'),
			maximum: Number(match[maximumGroup] ?? '0')
		});
	}
	return m['vms.create.errorCreation']();
}

/** Parses the server's out-of-range messages into a localized string. The
 *  server emits "%w: cpuCores must be between %d and %d" (and memoryMB/disk
 *  sizeGB variants) - server/internal/vm/create.go. */
function translateOutOfRange(message: string): string {
	const match = message.match(/^(\w+) must be between (\d+) and (\d+)$/);
	if (!match) return m['vms.create.errorCreation']();
	const field = match[1] ?? '';
	const min = match[2] ?? '0';
	const max = match[3] ?? '0';
	const fieldLabel = GABARIT_FIELD_LABELS[field]?.() ?? field;
	return m['vms.create.errorOutOfRange']({ field: fieldLabel, min: Number(min), max: Number(max) });
}

/** Server error codes whose message body needs structured parsing into a
 *  localized string (the server sends free text, not structured fields). */
const DYNAMIC_SUBMIT_ERRORS: Record<string, (message: string) => string> = {
	capacity_exceeded: translateCapacityExceeded,
	quota_exceeded: translateQuotaExceeded,
	gabarit_exceeded: translateGabaritExceeded,
	out_of_range: translateOutOfRange
};

/** "not_approved" messages (server/internal/vm/create.go) all share the
 *  "not approved for this cluster: <detail>" shape - parsed here since the
 *  server sends free text, not a structured code per resource kind. */
const NOT_APPROVED_DETAILS: Array<{ re: RegExp; translate: (match: RegExpMatchArray) => string }> = [
	{
		re: /^template vmid (\d+) is not approved for this cluster$/,
		translate: (match) => m['vms.create.errorNotApprovedTemplate']({ template: match[1] ?? '' })
	},
	{
		re: /^profile "(.+)" is not approved for this cluster$/,
		translate: (match) => m['vms.create.errorNotApprovedProfile']({ profile: match[1] ?? '' })
	},
	{ re: /^no approved node in catalog$/, translate: () => m['vms.create.errorNoApprovedNode']() },
	{
		re: /^no approved storage on node "(.+)"$/,
		translate: (match) => m['vms.create.errorNoApprovedStorageOnNode']({ node: match[1] ?? '' })
	},
	{
		re: /^no approved bridge on node "(.+)"$/,
		translate: (match) => m['vms.create.errorNoApprovedBridgeOnNode']({ node: match[1] ?? '' })
	},
	{
		re: /^network model "(.+)"$/,
		translate: (match) => m['vms.create.errorNotApprovedNetworkModel']({ model: match[1] ?? '' })
	},
	{
		re: /^storage "(.+)" on node "(.+)"$/,
		translate: (match) => m['vms.create.errorNotApprovedStorage']({ storage: match[1] ?? '', node: match[2] ?? '' })
	},
	{
		re: /^bridge "(.+)" on node "(.+)"$/,
		translate: (match) => m['vms.create.errorNotApprovedBridge']({ bridge: match[1] ?? '', node: match[2] ?? '' })
	},
	{
		re: /^iso "(.+)" on storage "(.+)" on node "(.+)"$/,
		translate: (match) => m['vms.create.errorNotApprovedIso']({ file: match[1] ?? '', storage: match[2] ?? '', node: match[3] ?? '' })
	},
	{
		re: /^no approved node holds iso "(.+)" on storage "(.+)"$/,
		translate: (match) => m['vms.create.errorNoNodeHoldsIso']({ file: match[1] ?? '', storage: match[2] ?? '' })
	},
	{
		re: /^node "(.+)"$/,
		translate: (match) => m['vms.create.errorNotApprovedNode']({ node: match[1] ?? '' })
	}
];

function translateNotApproved(message: string): string {
	const detail = message.replace(/^not approved for this cluster: /, '');
	for (const { re, translate } of NOT_APPROVED_DETAILS) {
		const match = detail.match(re);
		if (match) return translate(match);
	}
	return m['vms.create.errorNotApprovedGeneric']();
}

/** Translates a VM-create submit failure - never shows the server's raw
 *  English message (constitution: UI text is always localized). */
function translateSubmitError(error: unknown): string {
	if (!(error instanceof ApiRequestError)) return m['vms.create.errorCreation']();
	if (error.code === 'not_approved') return translateNotApproved(error.serverMessage);
	const dynamic = DYNAMIC_SUBMIT_ERRORS[error.code];
	if (dynamic) return dynamic(error.serverMessage);
	return (FIXED_SUBMIT_ERRORS[error.code] ?? m['vms.create.errorCreation'])();
}

/**
 * Create-form state (V08/V09): the form's own local edits are ordinary
 * mutable $state; the fetched catalog is an API response and stays $state.raw
 * (constitution VII). One store instance per create screen, via context.
 */
export class VmCreateStore {
	catalog = $state.raw<VmCreateCatalog | null>(null);
	catalogError = $state.raw<string | null>(null);
	submitting = $state.raw(false);
	submitError = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state.raw('');

	mode = $state<CreateMode>('simple');
	name = $state('');
	profileId = $state('');
	node = $state('');
	nodeAdjusted = $state(false);
	storage = $state('');
	storageAdjusted = $state(false);
	tagsInput = $state('');
	cpuCores = $state(1);
	sockets = $state(1);
	memoryMB = $state(2048);
	diskSizeGB = $state(20);
	diskStorage = $state('');
	/** Multi-NIC rows (US2/D3a). Simple mode uses nics[0] only; detailed
	 *  mode allows add/remove up to gabarit.maxNetworkCards. */
	nics = $state<NICRow[]>([{ bridge: '', model: 'virtio' }]);
	isoFile = $state('');
	/** US2/issue-02: the VM source - 'iso' (default, preserves existing
	 *  behaviour) or 'template' (clone from an approved Proxmox template).
	 *  When 'template' is selected, the node selector is hidden (D2b: the
	 *  clone stays on the template's node). */
	sourceType = $state<VmSource>('iso');
	/** Simple-mode source (V08): 'profile' (predefined size) or 'template'
	 *  (clone from an approved Proxmox template). Profile is the default
	 *  because it matches the original simple workflow. */
	simpleSource = $state<SimpleSource>('profile');
	templateId = $state(0);
	/** Issue 04: the selected template's disk floor. 0 when no template is
	 *  selected - the disk size may never drop below it (Proxmox cannot
	 *  shrink a clone's source disk). */
	templateMinDiskGB = $state(0);
	startAfterCreate = $state(true);
	/** US6/issue-06: UEFI (bios=ovmf + q35 + efidisk0) and TPM 2.0 -
	 *  available in both modes. UEFI defaults on (modern OSes expect UEFI
	 *  boot); TPM stays opt-in and requires UEFI (the server rejects
	 *  TPM-without-UEFI with ErrInvalidRequest). There is no Secure Boot
	 *  option: the EFI disk is always created with an empty key store, since
	 *  most Linux ISOs ship an unsigned bootloader that Secure Boot refuses
	 *  to run, leaving the guest stuck in the UEFI shell. */
	uefi = $state(true);
	tpm = $state(false);
	/** The user's machines on the selected cluster (vmid + name), loaded
	 *  with the catalog so the form can refuse a duplicate name before
	 *  submitting (DESIGN.md §7, no duplicate on an uncertain outcome). The
	 *  server's name_taken check stays the real guard. */
	existingMachines = $state.raw<readonly { vmid: number; name: string }[]>([]);

	/** Fetches the multi-cluster options and defaults to the first one, matching
	 *  the login page's cluster picker (must run before loadCatalog when the
	 *  deployment has more than one cluster - the catalog needs one to target). */
	async loadClusters(principal: { isAdmin: boolean; cluster: string } | null = null): Promise<void> {
		try {
			this.clusterOptions = pinnedClusterOptions(await fetchClusterOptions(), principal);
		} catch {
			this.clusterOptions = [];
		}
		const first = this.clusterOptions[0];
		if (first && this.cluster === '') this.cluster = first.name;
	}

	setCluster(name: string): void {
		this.cluster = name;
		void this.loadCatalog();
		void this.loadExistingMachines();
	}

	/** The cluster's human-readable name for display - never the raw internal
	 *  id (which may be an opaque string like "default" that means nothing to
	 *  the operator; the id is only ever meaningful as an API parameter). */
	clusterDisplayName(): string {
		const clusterId = this.catalog?.cluster ?? this.cluster;
		const option = this.clusterOptions.find((candidate) => candidate.name === clusterId);
		return option?.displayName || clusterId;
	}

	/** Fetch the approved-resource catalog for the selected cluster (FR-002). */
	async loadCatalog(): Promise<void> {
		this.catalogError = null;
		try {
			const path =
				this.cluster === ''
					? '/api/v1/vm-create/catalog'
					: `/api/v1/vm-create/catalog?cluster=${encodeURIComponent(this.cluster)}`;
			this.catalog = await get<VmCreateCatalog>(path);
		} catch (error: unknown) {
			this.catalogError = error instanceof ApiRequestError ? error.message : m['vms.create.errorCatalog']();
		}
	}

	/** Loads the names of the user's machines on the selected cluster.
	 *  Best-effort: a failure leaves the list empty and the server check
	 *  answers instead. */
	async loadExistingMachines(): Promise<void> {
		try {
			const query = this.cluster === '' ? '' : `&cluster=${encodeURIComponent(this.cluster)}`;
			const result = await get<{ items: { vmid: number; name: string; cluster: string }[] }>(`/api/v1/vms?pageSize=100${query}`);
			this.existingMachines = result.items
				.filter((item) => this.cluster === '' || item.cluster === this.cluster)
				.map((item) => ({ vmid: item.vmid, name: item.name }));
		} catch {
			this.existingMachines = [];
		}
	}

	/** The existing machine with this name, if any (case-insensitive, like
	 *  a hostname). */
	machineNamed(name: string): { vmid: number; name: string } | null {
		const wanted = name.trim().toLowerCase();
		if (wanted === '') return null;
		return this.existingMachines.find((machine) => machine.name.toLowerCase() === wanted) ?? null;
	}

	/** The default node shown when the user did not adjust it (FR-010). The
	 *  server picks by capacity, so this is only exact once sent explicitly. */
	autoNode(): string {
		return this.catalog?.nodes[0] ?? '';
	}

	/** The storage the server will pick for a node when not adjusted (FR-010). */
	autoStorage(node: string): string {
		const match = this.catalog?.storages.find((storage) => storage.node === node);
		return match?.name ?? '';
	}

	effectiveNode(): string {
		// A clone stays on its template's node (templateId is reset when the source changes).
		const template = this.templateId !== 0 ? this.catalog?.templates.find((t) => t.vmid === this.templateId) : undefined;
		if (template) return template.node;
		return this.nodeAdjusted && this.node !== '' ? this.node : this.autoNode();
	}

	effectiveStorage(): string {
		if (this.storageAdjusted && this.storage !== '') return this.storage;
		return this.autoStorage(this.effectiveNode());
	}

	/** Parses tagsInput's comma-separated string into its selected tag names
	 *  (detailed mode's tag picker - FR-014: admin-created tags only). */
	selectedTags(): string[] {
		return this.tagsInput
			.split(',')
			.map((tag) => tag.trim())
			.filter((tag) => tag !== '');
	}

	/** Toggles one catalog tag in/out of tagsInput, preserving the others. */
	toggleTag(name: string): void {
		const current = this.selectedTags();
		const next = current.includes(name) ? current.filter((tag) => tag !== name) : [...current, name];
		this.tagsInput = next.join(', ');
	}

	/** Adds a NIC row in detailed mode, up to gabarit.maxNetworkCards (US2/D3a). */
	addNIC(): void {
		const max = this.catalog?.gabarit?.maxNetworkCards ?? 4;
		if (max !== 0 && this.nics.length >= max) return;
		this.nics.push({ bridge: '', model: 'virtio' });
	}

	/** Removes a NIC row at the given index (US2/D3a). At least one row remains. */
	removeNIC(index: number): void {
		if (this.nics.length <= 1) return;
		this.nics.splice(index, 1);
	}

	/** The configured capacité for a node, if the administrator set one. */
	nodeCapacity(node: string): CatalogNodeCapacity | undefined {
		return this.catalog?.nodeCapacities?.find((capacity) => capacity.node === node);
	}

	/** Selects the detailed-mode VM source, clearing the other sources
	 *  (US2/issue-02 D2a mutual exclusion). */
	setSourceType(source: VmSource): void {
		this.sourceType = source;
		if (source !== 'iso') this.isoFile = '';
		if (source !== 'template') {
			this.templateId = 0;
			this.templateMinDiskGB = 0;
		}
	}

	/** Selects the simple-mode source, clearing the other sources. Unlike the
	 *  detailed source, an ISO is not a simple source of its own - it is an
	 *  optional add-on of the profile source, so only the non-profile sources
	 *  clear it. */
	setSimpleSource(source: SimpleSource): void {
		this.simpleSource = source;
		if (source !== 'template') {
			this.templateId = 0;
			this.templateMinDiskGB = 0;
		}
		if (source !== 'profile') this.isoFile = '';
	}

	/** True when the catalog has at least one profile. */
	hasProfiles(): boolean {
		return (this.catalog?.profiles ?? []).length > 0;
	}

	/** Builds the outgoing request for simple mode: profile-driven or a
	 *  template clone, with the auto-selections sent explicitly when the user
	 *  adjusted them (V08). */
	buildRequest(): VMCreateRequest {
		const request: VMCreateRequest = {
			cluster: this.catalog?.cluster ?? this.cluster,
			name: this.name.trim(),
			startAfterCreate: this.startAfterCreate
		};
		// Tags apply to every source and mode, so they are set before the branches.
		const tags = this.selectedTags();
		if (tags.length > 0) request.tags = tags;
		if (this.mode === 'simple') {
			if (this.simpleSource === 'template' && this.templateId !== 0) {
				request.templateId = this.templateId;
				// No uefi field: a clone inherits the template's own firmware.
				return request;
			}
			request.uefi = this.uefi;
			if (this.profileId !== '') request.profileId = this.profileId;
			const pinnedNode = this.nodeAdjusted ? this.effectiveNode() : '';
			if (pinnedNode !== '') request.node = pinnedNode;
			if (this.storageAdjusted && this.storage !== '') {
				request.disk = { storage: this.storage };
			}
			if (this.isoFile !== '') {
				// When the node is pinned, only an ISO on that node is valid
				// (the server rejects a mismatch with ErrNotApproved). When auto,
				// send any matching file - the server restricts candidate nodes
				// to those that hold the ISO (resolveResources/nodesWithISO).
				const iso = this.catalog?.isos.find(
					(entry) => entry.file === this.isoFile && (pinnedNode === '' || entry.node === pinnedNode)
				);
				if (iso !== undefined) request.iso = { storage: iso.storage, file: iso.file };
			}
			return request;
		}
		return this.buildDetailedRequest(request);
	}

	/** Detailed mode: every field explicit (FR-011). The source is an ISO
	 *  or a Proxmox template (US2/issue-02 D2a): when sourceType is
	 *  'template', templateId is sent and the ISO field is omitted; the node
	 *  is derived from the template (D2b) for clones, not sent. */
	buildDetailedRequest(request: VMCreateRequest): VMCreateRequest {
		request.sockets = this.sockets;
		request.cpuCores = this.cpuCores;
		request.memoryMB = this.memoryMB;
		request.disk = { storage: this.diskStorage, sizeGB: this.diskSizeGB };
		request.network = this.nics.map((nic) => ({ bridge: nic.bridge, model: nic.model }));

		if (this.sourceType === 'template' && this.templateId !== 0) {
			request.templateId = this.templateId;
			// D2b: the node is derived from the template server-side; do not
			// send a client-supplied node for template clones.
		} else {
			request.node = this.node;
			if (this.isoFile !== '') {
				const iso = this.catalog?.isos.find((entry) => entry.file === this.isoFile && entry.node === this.node);
				if (iso !== undefined) request.iso = { storage: iso.storage, file: iso.file };
			}
		}

		// US6/issue-06: UEFI is sent explicitly (true or false) so an
		// unchecked box is honored - the server defaults to UEFI=true when
		// the field is absent entirely (simple mode's template branch never
		// sends it). TPM is only sent when true; the server default is false.
		request.uefi = this.uefi;
		if (this.uefi && this.tpm) request.tpm = true;

		return request;
	}

	/** Submits the built request; returns the accepted task handle on 202. */
	async submit(): Promise<VmCreateAccepted | null> {
		// Issue 04: the client already knows the template's disk floor - 
		// reject below it locally instead of a round-trip to ErrDiskReduction.
		if (this.templateMinDiskGB > 0 && this.diskSizeGB < this.templateMinDiskGB) {
			this.submitError = m['vms.create.diskBelowTemplateMin']({ min: this.templateMinDiskGB });
			return null;
		}

		this.submitting = true;
		this.submitError = null;
		try {
			const request: VMCreateRequest = this.buildRequest();
			const accepted: VmCreateAccepted = await post<VmCreateAccepted>('/api/v1/vms', request);
			return { ...accepted, expectedStatus: request.startAfterCreate ? 'running' : 'stopped' };
		} catch (error: unknown) {
			this.submitError = translateSubmitError(error);
			return null;
		} finally {
			this.submitting = false;
		}
	}
}

const VM_CREATE_CONTEXT_KEY = Symbol('vm-create');

/** Called once, by the create route. */
export function setVmCreateContext(): VmCreateStore {
	const store = new VmCreateStore();
	setContext(VM_CREATE_CONTEXT_KEY, store);
	return store;
}

export function getVmCreateContext(): VmCreateStore {
	return getContext<VmCreateStore>(VM_CREATE_CONTEXT_KEY);
}
