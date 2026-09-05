import { getContext, setContext } from 'svelte';
import { get, post, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { m } from '$lib/paraglide/messages.js';
import type { DraftValues } from './draft.svelte';

export interface CatalogStorage {
	name: string;
	node: string;
}

/** One approved bridge on one node — bridge approval is per-node, like
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

/** One approved cloud image (import-from source). SizeBytes lets the UI
 *  enforce the minimum disk size — the server rejects a smaller disk with
 *  "disk_below_image" (Proxmox import-from grows but never shrinks). */
export interface CatalogImage {
	storage: string;
	node: string;
	file: string;
	sizeBytes: number;
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

export interface CatalogCloudInitTemplate {
	id: string;
	label: string;
}

/** One approved Proxmox template (US2/issue-02). The VMID is the Proxmox
 *  VMID of the template VM; the node determines where the clone lands
 *  (D2b: cross-node clone is forbidden). CloudInitCapable signals the UI
 *  that the template supports cloud-init. DiskSizeGB is the template's
 *  disk size (reductions are rejected). DiskStorage is where the template's
 *  disk lives — the clone *source* storage, used to warn when the target
 *  storage differs (full copy instead of linked clone). */
export interface CatalogTemplate {
	vmid: number;
	node: string;
	name: string;
	cloudInitCapable: boolean;
	diskSizeGB: number;
	diskStorage: string;
}

export interface CatalogTag {
	name: string;
	color: string;
}

/** The administrator-editable per-VM size ceiling (gabarit) — the same
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

/** The caller's own VM count against the cluster's per-user allowance.
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
	images: CatalogImage[];
	profiles: CatalogProfile[];
	templates: CatalogTemplate[];
	cloudInitTemplates: CatalogCloudInitTemplate[];
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

/** The cloud-init configuration of a cloud-image creation (image mode),
 *  delivered entirely through Proxmox's native cloud-init keys — the REST
 *  API cannot write a per-VM snippet file, so there is no packages or raw
 *  user-data field; a fixed, admin-preplaced baseline snippet is attached
 *  server-side when present. No password field either — access is via SSH
 *  keys; a password is set post-boot through the guest agent. */
export interface ImageCloudInitRequest {
	user?: string;
	sshKeys?: string[];
	ipMode?: 'dhcp' | 'static';
	ipAddress?: string;
	gateway?: string;
}

/** One cloud-image source in the creation request: the image is imported as
 *  the VM's primary disk (import-from) and configured by cloud-init on first
 *  boot. CloudInit is mandatory in image mode — a cloud image has no
 *  installer, so cloud-init is the only way in. */
export interface ImageRequest {
	storage: string;
	file: string;
	cloudInit: ImageCloudInitRequest;
}

/** The single request shape both modes POST (FR-001) — no pool, no mode.
 *  The VM source is either an ISO (iso field), a Proxmox template
 *  (templateId field) or a cloud image (image field), never more than one
 *  (US2/issue-02 D2a). */
export interface VMCreateRequest {
	cluster: string;
	name: string;
	profileId?: string;
	cloudInitTemplateId?: string;
	node?: string;
	tags?: string[];
	sockets?: number;
	cpuCores?: number;
	memoryMB?: number;
	disk?: { storage?: string; sizeGB?: number };
	network?: NICRequest[];
	iso?: { storage: string; file: string };
	templateId?: number;
	image?: ImageRequest;
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
	cloudInitTemplateId?: string;
	cloudInitPushError?: string;
}

export type CreateMode = 'simple' | 'detailed';

/** The VM source type (US2/issue-02 D2a): 'iso' for OS without cloud images
 *  (Windows, appliances), 'template' for cloud-init-capable Proxmox
 *  templates, 'image' for approved cloud images imported with cloud-init.
 *  The three are mutually exclusive — the server rejects a request carrying
 *  more than one. */
export type VmSource = 'iso' | 'template' | 'image';

/** Simple-mode source (V08): the user can either pick a size profile, clone
 *  from an approved Proxmox template, or boot an approved cloud image. This
 *  is a UI-only distinction; the request still carries profileId,
 *  templateId or image. */
export type SimpleSource = 'profile' | 'template' | 'image';

/** Conversion factor for disk-size checks — mirrors the server's
 *  bytesPerGB (server/internal/vm/create.go). */
const BYTES_PER_GB = 1024 * 1024 * 1024;

/** Server error codes with fixed, non-parameterized text (server/internal/httpapi/vm_create.go). */
const FIXED_SUBMIT_ERRORS: Partial<Record<string, () => string>> = {
	admin_cannot_create: m['vms.create.adminBlocked'],
	no_pool: m['vms.create.errorNoPool'],
	invalid_name: m['vms.create.errorInvalidName'],
	name_taken: m['vms.create.errorNameTaken'],
	invalid_source: m['vms.create.errorInvalidSource'],
	invalid_request: m['vms.create.errorInvalidRequest'],
	disk_reduction: m['vms.create.errorDiskReduction'],
	disk_below_image: m['vms.create.errorDiskBelowImage'],
	insufficient_disk_space: m['vms.create.errorInsufficientDiskSpace'],
	no_snippet_storage: m['vms.create.errorNoSnippetStorage'],
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

/** Parses the server's "%s already owns %d of %d allowed VMs" message into a
 *  localized string. */
function translateQuotaExceeded(message: string): string {
	const match = message.match(/already owns (\d+) of (\d+) allowed VMs$/);
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
 *  sizeGB variants) — server/internal/vm/create.go. */
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
 *  "not approved for this cluster: <detail>" shape — parsed here since the
 *  server sends free text, not a structured code per resource kind. */
const NOT_APPROVED_DETAILS: Array<{ re: RegExp; translate: (match: RegExpMatchArray) => string }> = [
	{
		re: /^cloud-init template "(.+)" is not approved for this cluster$/,
		translate: (match) => m['vms.create.errorNotApprovedTemplate']({ template: match[1] ?? '' })
	},
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

/** Translates a VM-create submit failure — never shows the server's raw
 *  English message (constitution: UI text is always localized). */
function translateSubmitError(error: unknown): string {
	if (!(error instanceof ApiRequestError)) return m['vms.create.errorCreation']();
	if (error.code === 'not_approved') return translateNotApproved(error.message);
	const dynamic = DYNAMIC_SUBMIT_ERRORS[error.code];
	if (dynamic) return dynamic(error.message);
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
	cloudInitTemplateId = $state('');
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
	/** US2/issue-02: the VM source — 'iso' (default, preserves existing
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
	 *  selected — the disk size may never drop below it (Proxmox cannot
	 *  shrink a clone's source disk). */
	templateMinDiskGB = $state(0);
	/** Cloud-image source (image mode): the selected image's storage and
	 *  file, keyed together because the same file name can exist on several
	 *  storages. Empty strings when no image is selected. */
	imageStorage = $state('');
	imageFile = $state('');
	/** The selected image's disk floor in GB (sizeBytes ceiled to GB) —
	 *  the disk size may never drop below it (server code "disk_below_image").
	 *  0 when no image is selected. */
	imageMinDiskGB = $state(0);
	/** Image-mode cloud-init (required): username, SSH public keys (one per
	 *  line), network mode with optional static addressing. Delivered
	 *  server-side through Proxmox's native cloud-init keys — no packages or
	 *  raw user-data field, since neither can be written per VM (a fixed,
	 *  admin-preplaced baseline snippet covers cluster-wide needs instead).
	 *  No password field — access is granted through SSH keys. */
	ciUser = $state('');
	ciSshKeysInput = $state('');
	ciIpMode = $state<'dhcp' | 'static'>('dhcp');
	ciIpAddress = $state('');
	ciGateway = $state('');
	startAfterCreate = $state(true);
	/** US6/issue-06: UEFI (bios=ovmf + q35 + efidisk0) and TPM 2.0 —
	 *  detailed-mode only. UEFI defaults on (modern OSes expect UEFI boot);
	 *  TPM stays opt-in. TPM requires UEFI; the server rejects
	 *  TPM-without-UEFI with ErrInvalidRequest. */
	uefi = $state(true);
	tpm = $state(false);

	/** Fetches the multi-cluster options and defaults to the first one, matching
	 *  the login page's cluster picker (must run before loadCatalog when the
	 *  deployment has more than one cluster — the catalog needs one to target). */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
		} catch {
			this.clusterOptions = [];
		}
		const first = this.clusterOptions[0];
		if (first && this.cluster === '') this.cluster = first.name;
	}

	setCluster(name: string): void {
		this.cluster = name;
		void this.loadCatalog();
	}

	/** The cluster's human-readable name for display — never the raw internal
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

	/** The node the server will pick when the user did not adjust it (FR-010). */
	autoNode(): string {
		return this.catalog?.nodes[0] ?? '';
	}

	/** The storage the server will pick for a node when not adjusted (FR-010). */
	autoStorage(node: string): string {
		const match = this.catalog?.storages.find((storage) => storage.node === node);
		return match?.name ?? '';
	}

	effectiveNode(): string {
		return this.nodeAdjusted && this.node !== '' ? this.node : this.autoNode();
	}

	effectiveStorage(): string {
		if (this.storageAdjusted && this.storage !== '') return this.storage;
		return this.autoStorage(this.effectiveNode());
	}

	/** Parses tagsInput's comma-separated string into its selected tag names
	 *  (detailed mode's tag picker — FR-014: admin-created tags only). */
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
		if (this.nics.length >= max) return;
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

	/** The catalog entry of the selected cloud image, if any. */
	selectedImage(): CatalogImage | undefined {
		if (this.imageFile === '') return undefined;
		return this.catalog?.images.find(
			(image) => image.file === this.imageFile && image.storage === this.imageStorage
		);
	}

	/** Selects the detailed-mode VM source, clearing the other sources
	 *  (US2/issue-02 D2a mutual exclusion). Image mode defaults
	 *  start-after-create to true: the VM is fully configured at first boot. */
	setSourceType(source: VmSource): void {
		this.sourceType = source;
		if (source !== 'iso') this.isoFile = '';
		if (source !== 'template') {
			this.templateId = 0;
			this.templateMinDiskGB = 0;
		}
		if (source !== 'image') this.clearImage();
		if (source === 'image') this.startAfterCreate = true;
	}

	/** Selects the simple-mode source, clearing the other sources. Image mode
	 *  defaults start-after-create to true (see setSourceType). Unlike the
	 *  detailed source, an ISO is not a simple source of its own — it is an
	 *  optional add-on of the profile source, so only the non-profile sources
	 *  clear it. */
	setSimpleSource(source: SimpleSource): void {
		this.simpleSource = source;
		if (source !== 'template') {
			this.templateId = 0;
			this.templateMinDiskGB = 0;
		}
		if (source !== 'profile') this.isoFile = '';
		if (source !== 'image') this.clearImage();
		if (source === 'image') this.startAfterCreate = true;
	}

	/** Selects a cloud image and derives its disk floor (sizeBytes ceiled to
	 *  GB, mirroring the server's checkDiskAboveImage). The image's node also
	 *  becomes the form's node so downstream selects filter correctly. */
	selectImage(storage: string, file: string): void {
		this.imageStorage = storage;
		this.imageFile = file;
		const image = this.selectedImage();
		this.imageMinDiskGB = image === undefined ? 0 : Math.ceil(image.sizeBytes / BYTES_PER_GB);
		if (image !== undefined) this.node = image.node;
	}

	/** Clears the cloud-image selection and its disk floor. */
	clearImage(): void {
		this.imageStorage = '';
		this.imageFile = '';
		this.imageMinDiskGB = 0;
	}

	/** Parses ciSshKeysInput's newline-separated textarea into the SSH key
	 *  list sent in the request. */
	sshKeys(): string[] {
		return this.ciSshKeysInput
			.split('\n')
			.map((key) => key.trim())
			.filter((key) => key !== '');
	}

	/** True when the active mode's source is a cloud image. */
	isImageSource(): boolean {
		return this.mode === 'simple' ? this.simpleSource === 'image' : this.sourceType === 'image';
	}

	/** True when the catalog has at least one profile — image mode requires
	 *  picking one when this is true (a fixed admin preset replaces the tiny
	 *  1 vCPU/128 MB default cloud images used to get). */
	hasProfiles(): boolean {
		return (this.catalog?.profiles ?? []).length > 0;
	}

	/** The client-side blocker for image mode, or null when the form may be
	 *  submitted: an image must be selected, a profile is required when the
	 *  cluster has any (otherwise the disk must cover the image), and
	 *  cloud-init is mandatory (user, SSH keys, static IP when static). */
	imageModeBlocker(): string | null {
		if (!this.isImageSource()) return null;
		if (this.imageFile === '') return m['vms.create.errorImageRequired']();
		if (this.hasProfiles()) {
			if (this.profileId === '' || !this.catalog?.profiles.some((profile) => profile.id === this.profileId)) {
				return m['vms.create.errorProfileRequired']();
			}
		} else if (this.diskSizeGB < this.imageMinDiskGB) {
			return m['vms.create.diskBelowImageMin']({ min: this.imageMinDiskGB });
		}
		if (this.ciUser.trim() === '') return m['vms.create.errorCiUserRequired']();
		if (this.sshKeys().length === 0) return m['vms.create.errorCiSshKeysRequired']();
		if (this.ciIpMode === 'static' && this.ciIpAddress.trim() === '') {
			return m['vms.create.errorCiIpRequired']();
		}
		return null;
	}

	/** Builds the image source of the outgoing request (image mode). */
	buildImageRequest(): ImageRequest {
		const isStatic = this.ciIpMode === 'static';
		const cloudInit: ImageCloudInitRequest = {
			user: this.ciUser.trim(),
			sshKeys: this.sshKeys(),
			ipMode: this.ciIpMode
		};
		if (isStatic) {
			cloudInit.ipAddress = this.ciIpAddress.trim();
			if (this.ciGateway.trim() !== '') cloudInit.gateway = this.ciGateway.trim();
		}
		return { storage: this.imageStorage, file: this.imageFile, cloudInit };
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
		if (this.mode === 'simple') {
			// Image mode (cloud image): the image is the source, the disk size
			// is required and must cover the image; storage is auto-selected
			// server-side and the node among the ones holding the image.
			if (this.simpleSource === 'image' && this.imageFile !== '') {
				request.image = this.buildImageRequest();
				if (this.profileId !== '') {
					request.profileId = this.profileId;
				} else {
					request.disk = { sizeGB: this.diskSizeGB };
				}
				return request;
			}
			if (this.simpleSource === 'template' && this.templateId !== 0) {
				request.templateId = this.templateId;
				if (this.cloudInitTemplateId !== '') request.cloudInitTemplateId = this.cloudInitTemplateId;
				return request;
			}
			if (this.profileId !== '') request.profileId = this.profileId;
			if (this.cloudInitTemplateId !== '') request.cloudInitTemplateId = this.cloudInitTemplateId;
			if (this.nodeAdjusted && this.node !== '') request.node = this.node;
			if (this.storageAdjusted && this.storage !== '') {
				request.disk = { storage: this.storage };
			}
			if (this.isoFile !== '') {
				// When the node is adjusted, only an ISO on that node is valid
				// (the server rejects a mismatch with ErrNotApproved). When auto,
				// send any matching file — the server restricts candidate nodes
				// to those that hold the ISO (resolveResources/nodesWithISO).
				const iso = this.catalog?.isos.find(
					(entry) => entry.file === this.isoFile && (!this.nodeAdjusted || entry.node === this.node)
				);
				if (iso !== undefined) request.iso = { storage: iso.storage, file: iso.file };
			}
			return request;
		}
		return this.buildDetailedRequest(request);
	}

	/** Detailed mode: every field explicit (FR-011) — except image source,
	 *  which may carry a profileId (server ignores the explicit hardware
	 *  fields when one is set, FR-009). The source is an ISO, a Proxmox
	 *  template or a cloud image (US2/issue-02 D2a): when sourceType is
	 *  'template', templateId is sent and the ISO field is omitted; when
	 *  'image', the image and its cloud-init are sent; the node is derived
	 *  from the template (D2b) for clones, not sent. */
	buildDetailedRequest(request: VMCreateRequest): VMCreateRequest {
		request.sockets = this.sockets;
		request.cpuCores = this.cpuCores;
		request.memoryMB = this.memoryMB;
		request.disk = { storage: this.diskStorage, sizeGB: this.diskSizeGB };
		request.network = this.nics.map((nic) => ({ bridge: nic.bridge, model: nic.model }));
		const tags = this.tagsInput
			.split(',')
			.map((tag) => tag.trim())
			.filter((tag) => tag !== '');
		if (tags.length > 0) request.tags = tags;

		if (this.sourceType === 'template' && this.templateId !== 0) {
			request.templateId = this.templateId;
			// D2b: the node is derived from the template server-side; do not
			// send a client-supplied node for template clones.
		} else {
			request.node = this.node;
			if (this.sourceType === 'image' && this.imageFile !== '') {
				request.image = this.buildImageRequest();
				if (this.profileId !== '') request.profileId = this.profileId;
			} else if (this.isoFile !== '') {
				const iso = this.catalog?.isos.find((entry) => entry.file === this.isoFile && entry.node === this.node);
				if (iso !== undefined) request.iso = { storage: iso.storage, file: iso.file };
			}
		}

		// US6/issue-06: UEFI/TPM are detailed-mode-only options. UEFI is sent
		// explicitly (true or false) so an unchecked box is honored — the
		// server defaults to UEFI=true when the field is absent entirely
		// (simple mode never sends it). TPM only sent when true; the server
		// default there is already false.
		request.uefi = this.uefi;
		if (this.uefi && this.tpm) request.tpm = true;

		return request;
	}

	/** Submits the built request; returns the accepted task handle on 202. */
	async submit(): Promise<VmCreateAccepted | null> {
		// Issue 04: the client already knows the template's disk floor —
		// reject below it locally instead of a round-trip to ErrDiskReduction.
		if (this.templateMinDiskGB > 0 && this.diskSizeGB < this.templateMinDiskGB) {
			this.submitError = m['vms.create.diskBelowTemplateMin']({ min: this.templateMinDiskGB });
			return null;
		}

		// Image mode: enforce the image's disk floor and the mandatory
		// cloud-init client-side (the server re-checks both).
		const imageBlocker = this.imageModeBlocker();
		if (imageBlocker !== null) {
			this.submitError = imageBlocker;
			return null;
		}

		this.submitting = true;
		this.submitError = null;
		try {
			return await post<VmCreateAccepted>('/api/v1/vms', this.buildRequest());
		} catch (error: unknown) {
			this.submitError = translateSubmitError(error);
			return null;
		} finally {
			this.submitting = false;
		}
	}

	/** Snapshots every persistable field for the draft (FR-019). */
	snapshotValues(): DraftValues {
		return {
			mode: this.mode,
			name: this.name,
			profileId: this.profileId,
			cloudInitTemplateId: this.cloudInitTemplateId,
			node: this.node,
			nodeAdjusted: this.nodeAdjusted,
			storage: this.storage,
			storageAdjusted: this.storageAdjusted,
			tagsInput: this.tagsInput,
			sockets: this.sockets,
			cpuCores: this.cpuCores,
			memoryMB: this.memoryMB,
			diskSizeGB: this.diskSizeGB,
			diskStorage: this.diskStorage,
			nics: this.nics.map((nic) => ({ ...nic })),
			isoFile: this.isoFile,
			sourceType: this.sourceType,
			simpleSource: this.simpleSource,
			templateId: this.templateId,
			templateMinDiskGB: this.templateMinDiskGB,
			imageStorage: this.imageStorage,
			imageFile: this.imageFile,
			imageMinDiskGB: this.imageMinDiskGB,
			ciUser: this.ciUser,
			ciSshKeysInput: this.ciSshKeysInput,
			ciIpMode: this.ciIpMode,
			ciIpAddress: this.ciIpAddress,
			ciGateway: this.ciGateway,
			startAfterCreate: this.startAfterCreate,
			uefi: this.uefi,
			tpm: this.tpm
		};
	}

	/** Restores a version-matched draft into the form (FR-020) — all fields
	 *  at once, never partially. */
	applyDraft(values: DraftValues): void {
		this.mode = values.mode;
		this.name = values.name;
		this.profileId = values.profileId;
		this.cloudInitTemplateId = values.cloudInitTemplateId ?? '';
		this.node = values.node;
		this.nodeAdjusted = values.nodeAdjusted;
		this.storage = values.storage;
		this.storageAdjusted = values.storageAdjusted;
		this.tagsInput = values.tagsInput;
		this.sockets = values.sockets ?? 1;
		this.cpuCores = values.cpuCores;
		this.memoryMB = values.memoryMB;
		this.diskSizeGB = values.diskSizeGB;
		this.diskStorage = values.diskStorage;
		this.nics = (values.nics ?? [{ bridge: '', model: 'virtio' }]).map((nic) => ({ ...nic }));
		this.isoFile = values.isoFile;
		this.sourceType = values.sourceType ?? 'iso';
		this.simpleSource = values.simpleSource ?? 'profile';
		this.templateId = values.templateId ?? 0;
		this.templateMinDiskGB = values.templateMinDiskGB ?? 0;
		this.imageStorage = values.imageStorage ?? '';
		this.imageFile = values.imageFile ?? '';
		this.imageMinDiskGB = values.imageMinDiskGB ?? 0;
		this.ciUser = values.ciUser ?? '';
		this.ciSshKeysInput = values.ciSshKeysInput ?? '';
		this.ciIpMode = values.ciIpMode ?? 'dhcp';
		this.ciIpAddress = values.ciIpAddress ?? '';
		this.ciGateway = values.ciGateway ?? '';
		this.startAfterCreate = values.startAfterCreate;
		this.uefi = values.uefi ?? true;
		this.tpm = values.tpm ?? false;
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
