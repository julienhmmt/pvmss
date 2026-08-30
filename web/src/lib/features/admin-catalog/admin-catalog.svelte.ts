import { get, post, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { setContext, getContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '$lib/paraglide/messages.js';

export interface AdminNode {
	name: string;
	status: string;
	cpuCores: number;
	cpuUsage: number;
	memoryTotal: number;
	memoryUsed: number;
	storageTotal: number;
	storageUsed: number;
	vmCount: number;
	enabled: boolean;
}

export interface AdminStorage {
	name: string;
	node: string;
	type: string;
	totalBytes: number;
	usedBytes: number;
	enabled: boolean;
}

export interface AdminBridge {
	name: string;
	node: string;
	active: boolean;
	comment: string;
	enabled: boolean;
}

export interface AdminISO {
	storage: string;
	node: string;
	file: string;
	sizeBytes: number;
	enabled: boolean;
}

interface ToggleResponse {
	name: string;
	enabled: boolean;
}

interface StorageToggleResponse {
	name: string;
	node: string;
	enabled: boolean;
}

interface BridgeToggleResponse {
	name: string;
	node: string;
	enabled: boolean;
}

interface ISOToggleResponse {
	node: string;
	storage: string;
	file: string;
	enabled: boolean;
}

/**
 * AdminCatalogStore manages the discover-and-approve state for nodes,
 * storages, bridges, and ISOs. One store instance per admin page, via context
 * (constitution VII: no module singletons). API responses are $state.raw —
 * they are API data, not form edits.
 */
export class AdminCatalogStore {
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');
	nodes = $state.raw<AdminNode[]>([]);
	storages = $state.raw<AdminStorage[]>([]);
	bridges = $state.raw<AdminBridge[]>([]);
	isos = $state.raw<AdminISO[]>([]);

	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	toggling = $state.raw<string | null>(null);
	toggleError = $state.raw<string | null>(null);

	isoSearch = $state('');
	isoStorageFilter = $state('');
	isoNodeFilter = $state('');
	isoEnabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	isoSortBy: ISOSortColumn = $state('file');
	isoSortDir: 'asc' | 'desc' = $state('asc');

	storageSearch = $state('');
	storageNodeFilter = $state('');
	storageTypeFilter = $state('');
	storageEnabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	storageSortBy: StorageSortColumn = $state('name');
	storageSortDir: 'asc' | 'desc' = $state('asc');

	bridgeSearch = $state('');
	bridgeNodeFilter = $state('');
	bridgeActiveFilter: 'all' | 'active' | 'inactive' = $state('all');
	bridgeEnabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	bridgeSortBy: BridgeSortColumn = $state('name');
	bridgeSortDir: 'asc' | 'desc' = $state('asc');

	nodeSearch = $state('');
	nodeStatusFilter = $state('');
	nodeEnabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	nodeSortBy: NodeSortColumn = $state('name');
	nodeSortDir: 'asc' | 'desc' = $state('asc');

	filteredIsos = $derived(
		sortIsos(
			this.isos.filter((iso) => {
				const search = this.isoSearch.toLowerCase();
				if (search && !iso.file.toLowerCase().includes(search)) return false;
				if (this.isoStorageFilter && iso.storage !== this.isoStorageFilter) return false;
				if (this.isoNodeFilter && iso.node !== this.isoNodeFilter) return false;
				if (this.isoEnabledFilter === 'enabled' && !iso.enabled) return false;
				if (this.isoEnabledFilter === 'disabled' && iso.enabled) return false;
				return true;
			}),
			this.isoSortBy,
			this.isoSortDir
		)
	);

	isoStorageOptions = $derived([...new SvelteSet(this.isos.map((i) => i.storage))].sort());
	isoNodeOptions = $derived([...new SvelteSet(this.isos.map((i) => i.node))].sort());

	filteredStorages = $derived(
		sortStorages(
			this.storages.filter((storage) => {
				const search = this.storageSearch.toLowerCase();
				if (search) {
					const match =
						storage.name.toLowerCase().includes(search) ||
						storage.node.toLowerCase().includes(search) ||
						storage.type.toLowerCase().includes(search);
					if (!match) return false;
				}
				if (this.storageNodeFilter && storage.node !== this.storageNodeFilter) return false;
				if (this.storageTypeFilter && storage.type !== this.storageTypeFilter) return false;
				if (this.storageEnabledFilter === 'enabled' && !storage.enabled) return false;
				if (this.storageEnabledFilter === 'disabled' && storage.enabled) return false;
				return true;
			}),
			this.storageSortBy,
			this.storageSortDir
		)
	);

	storageNodeOptions = $derived([...new SvelteSet(this.storages.map((s) => s.node))].sort());
	storageTypeOptions = $derived([...new SvelteSet(this.storages.map((s) => s.type))].sort());

	filteredBridges = $derived(
		sortBridges(
			this.bridges.filter((bridge) => {
				const search = this.bridgeSearch.toLowerCase();
				if (search) {
					const match =
						bridge.name.toLowerCase().includes(search) ||
						bridge.node.toLowerCase().includes(search) ||
						(bridge.comment || '').toLowerCase().includes(search);
					if (!match) return false;
				}
				if (this.bridgeNodeFilter && bridge.node !== this.bridgeNodeFilter) return false;
				if (this.bridgeActiveFilter === 'active' && !bridge.active) return false;
				if (this.bridgeActiveFilter === 'inactive' && bridge.active) return false;
				if (this.bridgeEnabledFilter === 'enabled' && !bridge.enabled) return false;
				if (this.bridgeEnabledFilter === 'disabled' && bridge.enabled) return false;
				return true;
			}),
			this.bridgeSortBy,
			this.bridgeSortDir
		)
	);

	bridgeNodeOptions = $derived([...new SvelteSet(this.bridges.map((b) => b.node))].sort());

	filteredNodes = $derived(
		sortNodes(
			this.nodes.filter((node) => {
				if (this.nodeSearch && !node.name.toLowerCase().includes(this.nodeSearch.toLowerCase())) return false;
				if (this.nodeStatusFilter && node.status !== this.nodeStatusFilter) return false;
				if (this.nodeEnabledFilter === 'enabled' && !node.enabled) return false;
				if (this.nodeEnabledFilter === 'disabled' && node.enabled) return false;
				return true;
			}),
			this.nodeSortBy,
			this.nodeSortDir
		)
	);

	nodeStatusFilterOptions = $derived([...new SvelteSet(this.nodes.map((n) => n.status))].sort());

	setStorageSort(column: StorageSortColumn): void {
		if (this.storageSortBy === column) {
			this.storageSortDir = this.storageSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.storageSortBy = column;
			this.storageSortDir = 'asc';
		}
	}

	setBridgeSort(column: BridgeSortColumn): void {
		if (this.bridgeSortBy === column) {
			this.bridgeSortDir = this.bridgeSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.bridgeSortBy = column;
			this.bridgeSortDir = 'asc';
		}
	}

	setISOSort(column: ISOSortColumn): void {
		if (this.isoSortBy === column) {
			this.isoSortDir = this.isoSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.isoSortBy = column;
			this.isoSortDir = 'asc';
		}
	}

	setNodeSort(column: NodeSortColumn): void {
		if (this.nodeSortBy === column) {
			this.nodeSortDir = this.nodeSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.nodeSortBy = column;
			this.nodeSortDir = 'asc';
		}
	}

	resetISOFilters(): void {
		this.isoSearch = '';
		this.isoStorageFilter = '';
		this.isoNodeFilter = '';
		this.isoEnabledFilter = 'all';
		this.isoSortBy = 'file';
		this.isoSortDir = 'asc';
	}

	resetStorageFilters(): void {
		this.storageSearch = '';
		this.storageNodeFilter = '';
		this.storageTypeFilter = '';
		this.storageEnabledFilter = 'all';
		this.storageSortBy = 'name';
		this.storageSortDir = 'asc';
	}

	resetBridgeFilters(): void {
		this.bridgeSearch = '';
		this.bridgeNodeFilter = '';
		this.bridgeActiveFilter = 'all';
		this.bridgeEnabledFilter = 'all';
		this.bridgeSortBy = 'name';
		this.bridgeSortDir = 'asc';
	}

	resetNodeFilters(): void {
		this.nodeSearch = '';
		this.nodeStatusFilter = '';
		this.nodeEnabledFilter = 'all';
		this.nodeSortBy = 'name';
		this.nodeSortDir = 'asc';
	}

	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			const first = this.clusterOptions[0];
			if (first && (this.clusterOptions.length === 1 || !this.clusterOptions.some((option) => option.name === this.cluster))) {
				this.cluster = first.name;
			}
		} catch (error: unknown) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.catalog.loadClustersError']();
		}
	}

	async loadAll(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		try {
			const [nodes, storages, bridges, isos] = await Promise.all([
				get<AdminNode[]>(`/api/v1/admin/nodes?cluster=${encodeURIComponent(this.cluster)}`),
				get<AdminStorage[]>(`/api/v1/admin/storages?cluster=${encodeURIComponent(this.cluster)}`),
				get<AdminBridge[]>(`/api/v1/admin/bridges?cluster=${encodeURIComponent(this.cluster)}`),
				get<AdminISO[]>(`/api/v1/admin/isos?cluster=${encodeURIComponent(this.cluster)}`)
			]);
			this.nodes = nodes;
			this.storages = storages;
			this.bridges = bridges;
			this.isos = isos;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.catalog.loadError']();
		} finally {
			this.loading = false;
		}
	}

	setCluster(value: string): void {
		this.cluster = value;
		void this.loadAll();
	}

	async toggleNode(name: string, enabled: boolean): Promise<void> {
		this.toggling = `node:${name}`;
		this.toggleError = null;
		try {
			await post<ToggleResponse>('/api/v1/admin/nodes/toggle', { cluster: this.cluster, name, enabled });
			this.nodes = this.nodes.map((n) => (n.name === name ? { ...n, enabled } : n));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.toggleNodeError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	async toggleStorage(name: string, node: string, enabled: boolean): Promise<void> {
		this.toggling = `storage:${name}@${node}`;
		this.toggleError = null;
		try {
			await post<StorageToggleResponse>('/api/v1/admin/storages/toggle', {
				cluster: this.cluster,
				name,
				node,
				enabled
			});
			this.storages = this.storages.map((s) =>
				s.name === name && s.node === node ? { ...s, enabled } : s
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.toggleStorageError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	async toggleBridge(node: string, name: string, enabled: boolean): Promise<void> {
		this.toggling = `bridge:${node}/${name}`;
		this.toggleError = null;
		try {
			await post<BridgeToggleResponse>('/api/v1/admin/bridges/toggle', {
				cluster: this.cluster,
				node,
				name,
				enabled
			});
			this.bridges = this.bridges.map((bridge) =>
				bridge.node === node && bridge.name === name ? { ...bridge, enabled } : bridge
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.toggleBridgeError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	async toggleISO(node: string, storage: string, file: string, enabled: boolean): Promise<void> {
		this.toggling = `iso:${node}:${storage}:${file}`;
		this.toggleError = null;
		try {
			await post<ISOToggleResponse>('/api/v1/admin/isos/toggle', {
				cluster: this.cluster,
				node,
				storage,
				file,
				enabled
			});
			this.isos = this.isos.map((i) =>
				i.node === node && i.storage === storage && i.file === file ? { ...i, enabled } : i
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.toggleIsoError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}
}

export type ISOSortColumn = 'file' | 'storage' | 'node' | 'size' | 'enabled';
export type StorageSortColumn = 'name' | 'node' | 'type' | 'usage' | 'enabled';
export type BridgeSortColumn = 'name' | 'node' | 'active' | 'comment' | 'enabled';
export type NodeSortColumn = 'name' | 'status' | 'vmCount' | 'cpuUsage' | 'memoryUsage' | 'enabled';

function sortBridges(bridges: AdminBridge[], sortBy: BridgeSortColumn, dir: 'asc' | 'desc'): AdminBridge[] {
	const sorted = [...bridges].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'name':
				cmp = a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
			case 'node':
				cmp = a.node.localeCompare(b.node) || a.name.localeCompare(b.name);
				break;
			case 'active':
				cmp = Number(a.active) - Number(b.active) || a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
			case 'comment':
				cmp = (a.comment || '').localeCompare(b.comment || '') || a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
			case 'enabled':
				cmp = Number(a.enabled) - Number(b.enabled) || a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}

function sortStorages(storages: AdminStorage[], sortBy: StorageSortColumn, dir: 'asc' | 'desc'): AdminStorage[] {
	const sorted = [...storages].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'name':
				cmp = a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
			case 'node':
				cmp = a.node.localeCompare(b.node) || a.name.localeCompare(b.name);
				break;
			case 'type':
				cmp = a.type.localeCompare(b.type) || a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
			case 'usage': {
				const aPct = a.totalBytes > 0 ? a.usedBytes / a.totalBytes : 0;
				const bPct = b.totalBytes > 0 ? b.usedBytes / b.totalBytes : 0;
				cmp = aPct - bPct || a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
			}
			case 'enabled':
				cmp = Number(a.enabled) - Number(b.enabled) || a.name.localeCompare(b.name) || a.node.localeCompare(b.node);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}

function sortIsos(isos: AdminISO[], sortBy: ISOSortColumn, dir: 'asc' | 'desc'): AdminISO[] {
	const sorted = [...isos].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'file':
				cmp = a.file.localeCompare(b.file);
				break;
			case 'storage':
				cmp = a.storage.localeCompare(b.storage) || a.file.localeCompare(b.file);
				break;
			case 'node':
				cmp = a.node.localeCompare(b.node) || a.file.localeCompare(b.file);
				break;
			case 'size':
				cmp = a.sizeBytes - b.sizeBytes || a.file.localeCompare(b.file);
				break;
			case 'enabled':
				cmp = Number(a.enabled) - Number(b.enabled) || a.file.localeCompare(b.file);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}

function sortNodes(nodes: AdminNode[], sortBy: NodeSortColumn, dir: 'asc' | 'desc'): AdminNode[] {
	const sorted = [...nodes].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'name':
				cmp = a.name.localeCompare(b.name);
				break;
			case 'status':
				cmp = a.status.localeCompare(b.status) || a.name.localeCompare(b.name);
				break;
			case 'vmCount':
				cmp = a.vmCount - b.vmCount || a.name.localeCompare(b.name);
				break;
			case 'cpuUsage':
				cmp = a.cpuUsage - b.cpuUsage || a.name.localeCompare(b.name);
				break;
			case 'memoryUsage': {
				const aPct = a.memoryTotal > 0 ? a.memoryUsed / a.memoryTotal : 0;
				const bPct = b.memoryTotal > 0 ? b.memoryUsed / b.memoryTotal : 0;
				cmp = aPct - bPct || a.name.localeCompare(b.name);
				break;
			}
			case 'enabled':
				cmp = Number(a.enabled) - Number(b.enabled) || a.name.localeCompare(b.name);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}

const ADMIN_CATALOG_CONTEXT_KEY = Symbol('admin-catalog');

export function setAdminCatalogContext(): AdminCatalogStore {
	const store = new AdminCatalogStore();
	setContext(ADMIN_CATALOG_CONTEXT_KEY, store);
	return store;
}

export function getAdminCatalogContext(): AdminCatalogStore {
	return getContext<AdminCatalogStore>(ADMIN_CATALOG_CONTEXT_KEY);
}
