import { get, post, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { setContext, getContext } from 'svelte';
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

	async toggleISO(storage: string, file: string, enabled: boolean): Promise<void> {
		this.toggling = `iso:${storage}:${file}`;
		this.toggleError = null;
		try {
			await post<ISOToggleResponse>('/api/v1/admin/isos/toggle', {
				cluster: this.cluster,
				storage,
				file,
				enabled
			});
			this.isos = this.isos.map((i) =>
				i.storage === storage && i.file === file ? { ...i, enabled } : i
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.toggleIsoError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}
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
