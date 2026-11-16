import { get, post, put, ApiRequestError } from '#lib/shared/api/client.js';
import {
	fetchClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from '#lib/shared/clusters.js';
import { getContext, setContext } from 'svelte';
import { m } from '#lib/paraglide/messages.js';
import type { NodeStatus } from '../cluster/nodes.svelte';

const REFRESH_POLL_MS = 1000;
const REFRESH_POLL_ATTEMPTS = 15;

export interface NodeCapacity {
	node: string;
	status: NodeStatus;
	maxVms: number;
	maxVcpus: number;
	maxRamGb: number;
	maxDiskGb: number;
	usedVms: number;
	usedVcpus: number;
	usedRamGb: number;
	usedDiskGb: number;
	physicalVcpus: number;
	physicalRamGb: number;
	/** Live node load covering every VM, not only pvmss-tagged ones. */
	nodeCpuUsage: number;
	nodeMemoryUsedGb: number;
	nodeStorageUsedGb: number;
	nodeStorageTotalGb: number;
	totalVms: number;
	/** Catalog approval state - false means the node is excluded from PVMSS
	 *  placement entirely, whatever its caps say. */
	approved: boolean;
}

export interface NodeCapacityPatch {
	maxVms: number;
	maxVcpus: number;
	maxRamGb: number;
	maxDiskGb: number;
}

/** Export file kind tag and versions: 1 carries one cluster's nodes, 2 a
 *  clusters[] bundle where each section names its target cluster. */
export const NODE_CAPACITY_EXPORT_KIND = 'pvmss-node-capacity';
export const NODE_CAPACITY_EXPORT_VERSION = 1;
export const NODE_CAPACITY_MULTI_EXPORT_VERSION = 2;

/** One node's caps as they travel through an export file. */
export type NodeCapacityExportItem = NodeCapacityPatch & { node: string };

/** One cluster section of an export file. cluster is null for v1 files: they
 *  carry a source label only and import into the cluster on screen. */
export interface NodeCapacityExportSection {
	cluster: string | null;
	nodes: NodeCapacityExportItem[];
}

export interface NodeCapacityImportOutcome {
	cluster: string;
	node: string;
	result: 'updated' | 'unchanged' | 'skipped' | 'not_found' | 'invalid' | 'failed';
	error?: string;
}

/** Nodes carrying at least one cap - the only ones an export can transfer. */
export function cappedNodes(nodes: NodeCapacity[]): NodeCapacity[] {
	return nodes.filter((node) => node.maxVms !== 0 || node.maxVcpus !== 0 || node.maxRamGb !== 0 || node.maxDiskGb !== 0);
}

function exportItems(nodes: NodeCapacity[]): NodeCapacityExportItem[] {
	return cappedNodes(nodes).map((node) => ({
		node: node.node,
		maxVms: node.maxVms,
		maxVcpus: node.maxVcpus,
		maxRamGb: node.maxRamGb,
		maxDiskGb: node.maxDiskGb
	}));
}

/** Serialises the cluster's configured caps to the export file content. */
export function buildNodeCapacityExport(cluster: string, nodes: NodeCapacity[]): string {
	return JSON.stringify({ kind: NODE_CAPACITY_EXPORT_KIND, version: NODE_CAPACITY_EXPORT_VERSION, cluster, nodes: exportItems(nodes) }, null, 2);
}

/** Serialises every cluster's configured caps to one export file; each
 *  section names the cluster its nodes are written back to on import. */
export function buildAllClustersNodeCapacityExport(sections: NodeCapacityExportSection[]): string {
	return JSON.stringify({ kind: NODE_CAPACITY_EXPORT_KIND, version: NODE_CAPACITY_MULTI_EXPORT_VERSION, clusters: sections }, null, 2);
}

/** Parses an export file (v1 single-cluster or v2 multi-cluster) into import
 *  sections. Every cap must be present: a missing one would silently uncap
 *  that dimension. Ranges are validated by the server. */
export function parseNodeCapacityExport(text: string): NodeCapacityExportSection[] {
	const data: unknown = JSON.parse(text);
	if (typeof data !== 'object' || data === null) throw new Error(m['policy.nodesImportInvalidFile']());
	const file = data as { kind?: unknown; version?: unknown; nodes?: unknown; clusters?: unknown };
	if (file.kind !== NODE_CAPACITY_EXPORT_KIND) throw new Error(m['policy.nodesImportInvalidFile']());
	if (file.version === NODE_CAPACITY_EXPORT_VERSION && Array.isArray(file.nodes)) {
		return [{ cluster: null, nodes: parseItems(file.nodes) }];
	}
	if (file.version === NODE_CAPACITY_MULTI_EXPORT_VERSION && Array.isArray(file.clusters)) {
		return file.clusters.map((section) => {
			const entry = section as { cluster?: unknown; nodes?: unknown };
			if (typeof entry !== 'object' || entry === null || typeof entry.cluster !== 'string' || !Array.isArray(entry.nodes)) {
				throw new Error(m['policy.nodesImportInvalidFile']());
			}
			return { cluster: entry.cluster, nodes: parseItems(entry.nodes) };
		});
	}
	throw new Error(m['policy.nodesImportInvalidFile']());
}

function parseItems(items: unknown[]): NodeCapacityExportItem[] {
	const fields = ['maxVms', 'maxVcpus', 'maxRamGb', 'maxDiskGb'];
	const complete = items.every(
		(item) =>
			typeof item === 'object' &&
			item !== null &&
			typeof (item as { node?: unknown }).node === 'string' &&
			fields.every((field) => Number.isInteger((item as Record<string, unknown>)[field]))
	);
	if (!complete) throw new Error(m['policy.nodesImportInvalidFile']());
	return items as NodeCapacityExportItem[];
}

interface NodeCapacityListResponse {
	nodes: NodeCapacity[];
	refreshedAt: string;
}

export type NodeCapacitySortColumn = 'node' | 'vms' | 'vcpus' | 'ram' | 'disk' | 'load';

/** Manages live node discovery joined with server-owned capacité values. */
export class AdminPolicyNodesStore {
	nodes = $state.raw<NodeCapacity[]>([]);
	refreshedAt = $state.raw<string | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	errorCode = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	saveErrorCode = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('');
	refreshing = $state.raw(false);
	refreshError = $state.raw<string | null>(null);
	refreshDisabled = $state.raw(false);

	#reenableTimer: ReturnType<typeof setTimeout> | null = null;
	/** Latest-wins guard for load: a cluster switch must not show the previous cluster's nodes. */
	#loadRequestId = 0;

	sortBy: NodeCapacitySortColumn = $state('node');
	sortDir: 'asc' | 'desc' = $state('asc');

	sortedNodes = $derived(sortNodeCapacities(this.nodes, this.sortBy, this.sortDir));

	setSort(column: NodeCapacitySortColumn): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
	}

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			if (this.clusterOptions.length > 0) {
				this.cluster = resolveInitialAdminCluster(this.clusterOptions, this.cluster);
			}
		} catch (error: unknown) {
			this.errorCode = error instanceof ApiRequestError ? error.code : null;
			this.error = error instanceof ApiRequestError ? error.message : m['policy.nodesLoadError']();
		}
	}

	setCluster(value: string): void {
		saveAdminCluster(value);
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		const requestId = ++this.#loadRequestId;
		await this.loadClusters();
		if (requestId !== this.#loadRequestId) return;
		this.loading = true;
		this.error = null;
		this.errorCode = null;
		try {
			const response = await get<NodeCapacityListResponse>(`/api/v1/admin/policy/nodes?cluster=${encodeURIComponent(this.cluster)}`);
			if (requestId !== this.#loadRequestId) return;
			this.nodes = response.nodes;
			this.refreshedAt = response.refreshedAt || null;
		} catch (error: unknown) {
			if (requestId !== this.#loadRequestId) return;
			this.errorCode = error instanceof ApiRequestError ? error.code : null;
			this.error = error instanceof ApiRequestError ? error.message : m['policy.nodesLoadError']();
		} finally {
			if (requestId === this.#loadRequestId) this.loading = false;
		}
	}

	async save(node: string, patch: NodeCapacityPatch): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.saveErrorCode = null;
		try {
			const updated = await put<NodeCapacity>(`/api/v1/admin/policy/nodes/${encodeURIComponent(node)}`, { cluster: this.cluster, ...patch });
			this.nodes = this.nodes.some((item) => item.node === node)
				? this.nodes.map((item) => (item.node === node ? updated : item))
				: [...this.nodes, updated];
		} catch (error: unknown) {
			this.saveErrorCode = error instanceof ApiRequestError ? error.code : null;
			this.saveError = error instanceof ApiRequestError ? error.message : m['policy.nodesSaveError']();
			throw error;
		} finally {
			this.saving = false;
		}
	}

	clearSaveError(): void {
		this.saveError = null;
		this.saveErrorCode = null;
	}

	/** Writes exported caps cluster section by cluster section (a v1 section
	 *  targets the cluster on screen) and reloads; the per-node report is
	 *  returned for the dialog. A refused section reports its nodes as failed
	 *  instead of stopping the others. */
	async importNodes(sections: NodeCapacityExportSection[], overwrite: boolean): Promise<NodeCapacityImportOutcome[] | null> {
		this.saving = true;
		this.saveError = null;
		this.saveErrorCode = null;
		const all: NodeCapacityImportOutcome[] = [];
		try {
			for (const section of sections) {
				if (section.nodes.length === 0) continue;
				const target = section.cluster ?? this.cluster;
				try {
					const outcomes = await post<Array<Omit<NodeCapacityImportOutcome, 'cluster'>>>('/api/v1/admin/policy/nodes/import', {
						cluster: target,
						overwrite,
						nodes: section.nodes
					});
					all.push(...outcomes.map((outcome) => ({ ...outcome, cluster: target })));
				} catch (error: unknown) {
					this.saveErrorCode = error instanceof ApiRequestError ? error.code : null;
					const message = error instanceof ApiRequestError ? error.message : m['policy.nodesImportError']();
					all.push(...section.nodes.map((item) => ({ cluster: target, node: item.node, result: 'failed' as const, error: message })));
				}
			}
			await this.load();
			return all;
		} finally {
			this.saving = false;
		}
	}

	/** Reads the configured caps of every cluster for a multi-cluster export
	 *  file; returns null and records the error when one cluster cannot be
	 *  read rather than writing a silently incomplete bundle. */
	async exportAllClusterCaps(): Promise<NodeCapacityExportSection[] | null> {
		try {
			const sections: NodeCapacityExportSection[] = [];
			for (const option of this.clusterOptions) {
				const response = await get<NodeCapacityListResponse>(`/api/v1/admin/policy/nodes?cluster=${encodeURIComponent(option.name)}`);
				sections.push({ cluster: option.name, nodes: exportItems(response.nodes) });
			}
			return sections;
		} catch (error: unknown) {
			this.saveError = error instanceof ApiRequestError ? error.message : m['policy.nodesExportError']();
			return null;
		}
	}

	async refresh(): Promise<void> {
		this.refreshing = true;
		this.refreshError = null;
		try {
			const previous = this.refreshedAt;
			await post('/api/v1/cluster/refresh');
			this.clearRefreshDisabled();
			await this.#reloadUntilRefreshed(previous);
		} catch (error: unknown) {
			if (error instanceof ApiRequestError && error.code === 'refresh_too_soon') {
				this.refreshDisabled = true;
				this.refreshError = error.message;
				this.#scheduleReenable(error.retryAfterSeconds ?? 5);
			} else {
				this.refreshError = error instanceof ApiRequestError ? error.message : m['nodes.errorRefresh']();
			}
		} finally {
			this.refreshing = false;
		}
	}

	async retryConnection(): Promise<void> {
		const previous = this.refreshedAt;
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// Ignore; the next load will surface the current state.
		}
		await this.#reloadUntilRefreshed(previous);
	}

	// The server answers 202 and refreshes in the background: re-read until
	// refreshedAt moves past `previous`, or give up after a bounded wait
	// (a failed refresh never advances it).
	async #reloadUntilRefreshed(previous: string | null): Promise<void> {
		for (let attempt = 0; attempt < REFRESH_POLL_ATTEMPTS; attempt++) {
			await this.load();
			if (this.error || this.refreshedAt !== previous) return;
			await new Promise((resolve) => setTimeout(resolve, REFRESH_POLL_MS));
		}
	}

	#scheduleReenable(seconds: number): void {
		if (this.#reenableTimer !== null) clearTimeout(this.#reenableTimer);
		this.#reenableTimer = setTimeout(() => {
			this.#reenableTimer = null;
			this.clearRefreshDisabled();
		}, seconds * 1000);
	}

	clearRefreshDisabled(): void {
		if (this.#reenableTimer !== null) {
			clearTimeout(this.#reenableTimer);
			this.#reenableTimer = null;
		}
		this.refreshDisabled = false;
		this.refreshError = null;
	}
}

/** Label lookup for server message words: request fields ("maxVcpus" from
 *  invalid_policy) and dimension words ("vcpu" from the cap errors). */
const ERROR_WORD_LABELS: Record<string, () => string> = {
	maxvms: () => m['policy.maxVms'](),
	maxvcpus: () => m['policy.maxVcpus'](),
	maxramgb: () => m['policy.maxRam'](),
	maxdiskgb: () => m['policy.maxNodeDisk'](),
	vms: () => m['policy.colVms'](),
	vcpu: () => m['policy.colVcpus'](),
	vcpus: () => m['policy.colVcpus'](),
	ram: () => m['policy.colRam'](),
	disk: () => m['policy.colDisk']()
};

function errorWordLabel(word: string): string {
	return ERROR_WORD_LABELS[word.toLowerCase()]?.() ?? word;
}

/** RAM and disk caps are expressed in GB - the unit joins the message so the
 *  translated sentence keeps the server's precision. */
function dimensionUnit(dimension: string): string {
	return dimension === 'ram' || dimension === 'disk' ? ` ${m['policy.unitGB']()}` : '';
}

/** Parses the server's free-text node-capacity errors into localized strings
 *  (server/internal/policy/admin.go - the server sends English text, not
 *  structured fields). Unknown shapes pass through unchanged. */
export function translateNodeCapacityError(code: string | null, message: string): string {
	const below = /^(\w+) cap \((\d+)\) is below (.+)'s current usage \((\d+)\)$/.exec(message);
	if (code === 'node_limit_below_usage' && below !== null) {
		const dimension = below[1] ?? '';
		return m['policy.errorCapBelowUsage']({
			dimension: errorWordLabel(dimension), requested: below[2] ?? '', node: below[3] ?? '',
			used: below[4] ?? '', unit: dimensionUnit(dimension)
		});
	}
	const above = /^(\w+) cap \((\d+)(?: GB)?\) exceeds (.+)'s physical capacity \((\d+)(?: GB)?\)$/.exec(message);
	if (code === 'node_limit_above_capacity' && above !== null) {
		const dimension = above[1] ?? '';
		return m['policy.errorCapAboveCapacity']({
			dimension: errorWordLabel(dimension), requested: above[2] ?? '', node: above[3] ?? '',
			physical: above[4] ?? '', unit: dimensionUnit(dimension)
		});
	}
	const negative = /^(\w+) must not be negative$/.exec(message);
	if (code === 'invalid_policy' && negative !== null) {
		return m['policy.errorNotNegative']({ field: errorWordLabel(negative[1] ?? '') });
	}
	return message;
}

const POLICY_NODES_CONTEXT_KEY = Symbol('admin-policy-nodes');

export function setAdminPolicyNodesContext(): AdminPolicyNodesStore {
	const store = new AdminPolicyNodesStore();
	setContext(POLICY_NODES_CONTEXT_KEY, store);
	return store;
}

export function getAdminPolicyNodesContext(): AdminPolicyNodesStore {
	return getContext<AdminPolicyNodesStore>(POLICY_NODES_CONTEXT_KEY);
}

/** Saturation of one dimension: distance to the cap that refuses creation.
 *  Uncapped dimensions report 0 - they can never block. */
function saturation(used: number, cap: number): number {
	return cap > 0 ? used / cap : 0;
}

function dimensionSaturation(node: NodeCapacity, dimension: 'vms' | 'vcpus' | 'ram' | 'disk'): number {
	switch (dimension) {
		case 'vms':
			return saturation(node.usedVms, node.maxVms);
		case 'vcpus':
			return saturation(node.usedVcpus, node.maxVcpus);
		case 'ram':
			return saturation(node.usedRamGb, node.maxRamGb);
		case 'disk':
			return saturation(node.usedDiskGb, node.maxDiskGb);
	}
}

/** Worst live-load ratio across CPU, memory, and storage (0 when unknown). */
function loadScore(node: NodeCapacity): number {
	const memory = node.physicalRamGb > 0 ? node.nodeMemoryUsedGb / node.physicalRamGb : 0;
	const storage = node.nodeStorageTotalGb > 0 ? node.nodeStorageUsedGb / node.nodeStorageTotalGb : 0;
	return Math.max(node.nodeCpuUsage, memory, storage);
}

function sortNodeCapacities(nodes: NodeCapacity[], sortBy: NodeCapacitySortColumn, dir: 'asc' | 'desc'): NodeCapacity[] {
	const sorted = [...nodes].sort((a, b) => {
		let cmp: number;
		if (sortBy === 'node') {
			cmp = a.node.localeCompare(b.node);
		} else if (sortBy === 'load') {
			cmp = loadScore(a) - loadScore(b) || a.node.localeCompare(b.node);
		} else {
			cmp = dimensionSaturation(a, sortBy) - dimensionSaturation(b, sortBy) || a.node.localeCompare(b.node);
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}
