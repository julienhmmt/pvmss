import { getContext, setContext } from 'svelte';
import { get, post, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type { DraftValues } from './draft.svelte';

export interface CatalogStorage {
	name: string;
	node: string;
}

export interface CatalogISO {
	storage: string;
	file: string;
}

export interface CatalogProfile {
	id: string;
	label: string;
	cpuCores: number;
	memoryMB: number;
	diskGB: number;
	bus: string;
}

export interface CatalogCloudInitTemplate {
	id: string;
	label: string;
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
	bridges: string[];
	isos: CatalogISO[];
	profiles: CatalogProfile[];
	cloudInitTemplates: CatalogCloudInitTemplate[];
	tags: CatalogTag[];
	gabarit?: CatalogGabarit;
	quota?: CatalogQuota;
	nodeCapacities?: CatalogNodeCapacity[];
}

/** The single request shape both modes POST (FR-001) — no pool, no mode. */
export interface VMCreateRequest {
	cluster: string;
	name: string;
	profileId?: string;
	cloudInitTemplateId?: string;
	node?: string;
	tags?: string[];
	cpuCores?: number;
	memoryMB?: number;
	disk?: { storage?: string; sizeGB?: number };
	network?: { bridge?: string; model?: string };
	iso?: { storage: string; file: string };
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

/** Server error codes with fixed, non-parameterized text (server/internal/httpapi/vm_create.go). */
const FIXED_SUBMIT_ERRORS: Partial<Record<string, () => string>> = {
	admin_cannot_create: m['vms.create.adminBlocked'],
	no_pool: m['vms.create.errorNoPool'],
	invalid_name: m['vms.create.errorInvalidName'],
	cluster_error: m['vms.create.errorClusterRejected'],
	internal_error: m['vms.create.errorInternal']
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
		re: /^iso "(.+)" on storage "(.+)"$/,
		translate: (match) => m['vms.create.errorNotApprovedIso']({ file: match[1] ?? '', storage: match[2] ?? '' })
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
	memoryMB = $state(2048);
	diskSizeGB = $state(20);
	diskStorage = $state('');
	bridge = $state('');
	networkModel = $state('virtio');
	isoFile = $state('');
	startAfterCreate = $state(true);

	/** Fetch the approved-resource catalog for the caller's cluster (FR-002). */
	async loadCatalog(): Promise<void> {
		this.catalogError = null;
		try {
			this.catalog = await get<VmCreateCatalog>('/api/v1/vm-create/catalog');
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

	/** The configured capacité for a node, if the administrator set one. */
	nodeCapacity(node: string): CatalogNodeCapacity | undefined {
		return this.catalog?.nodeCapacities?.find((capacity) => capacity.node === node);
	}

	/** Builds the outgoing request for simple mode: profile-driven, with the
	 *  auto-selections sent explicitly when the user adjusted them (V08). */
	buildRequest(): VMCreateRequest {
		const request: VMCreateRequest = {
			cluster: this.catalog?.cluster ?? 'default',
			name: this.name.trim(),
			startAfterCreate: this.startAfterCreate
		};
		if (this.mode === 'simple') {
			if (this.profileId !== '') request.profileId = this.profileId;
			if (this.cloudInitTemplateId !== '') request.cloudInitTemplateId = this.cloudInitTemplateId;
			if (this.nodeAdjusted && this.node !== '') request.node = this.node;
			if (this.storageAdjusted && this.storage !== '') {
				request.disk = { storage: this.storage };
			}
			return request;
		}
		return this.buildDetailedRequest(request);
	}

	/** Detailed mode: every field explicit (FR-011), no profile reference. */
	buildDetailedRequest(request: VMCreateRequest): VMCreateRequest {
		request.node = this.node;
		request.cpuCores = this.cpuCores;
		request.memoryMB = this.memoryMB;
		request.disk = { storage: this.diskStorage, sizeGB: this.diskSizeGB };
		request.network = { bridge: this.bridge, model: this.networkModel };
		const tags = this.tagsInput
			.split(',')
			.map((tag) => tag.trim())
			.filter((tag) => tag !== '');
		if (tags.length > 0) request.tags = tags;
		if (this.isoFile !== '') {
			const iso = this.catalog?.isos.find((entry) => entry.file === this.isoFile);
			if (iso !== undefined) request.iso = { storage: iso.storage, file: iso.file };
		}
		return request;
	}

	/** Submits the built request; returns the accepted task handle on 202. */
	async submit(): Promise<VmCreateAccepted | null> {
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
			cpuCores: this.cpuCores,
			memoryMB: this.memoryMB,
			diskSizeGB: this.diskSizeGB,
			diskStorage: this.diskStorage,
			bridge: this.bridge,
			networkModel: this.networkModel,
			isoFile: this.isoFile,
			startAfterCreate: this.startAfterCreate
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
		this.cpuCores = values.cpuCores;
		this.memoryMB = values.memoryMB;
		this.diskSizeGB = values.diskSizeGB;
		this.diskStorage = values.diskStorage;
		this.bridge = values.bridge;
		this.networkModel = values.networkModel;
		this.isoFile = values.isoFile;
		this.startAfterCreate = values.startAfterCreate;
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
