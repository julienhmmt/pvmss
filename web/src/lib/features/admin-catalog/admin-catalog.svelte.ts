import { get, post, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';

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
	nodes = $state.raw<AdminNode[]>([]);
	storages = $state.raw<AdminStorage[]>([]);
	bridges = $state.raw<AdminBridge[]>([]);
	isos = $state.raw<AdminISO[]>([]);

	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	toggling = $state.raw<string | null>(null);
	toggleError = $state.raw<string | null>(null);

	async loadAll(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const [nodes, storages, bridges, isos] = await Promise.all([
				get<AdminNode[]>('/api/v1/admin/nodes?cluster=default'),
				get<AdminStorage[]>('/api/v1/admin/storages?cluster=default'),
				get<AdminBridge[]>('/api/v1/admin/bridges?cluster=default'),
				get<AdminISO[]>('/api/v1/admin/isos?cluster=default')
			]);
			this.nodes = nodes;
			this.storages = storages;
			this.bridges = bridges;
			this.isos = isos;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : 'failed to load admin catalog';
		} finally {
			this.loading = false;
		}
	}

	async toggleNode(name: string, enabled: boolean): Promise<void> {
		this.toggling = `node:${name}`;
		this.toggleError = null;
		try {
			await post<ToggleResponse>('/api/v1/admin/nodes/toggle', { cluster: 'default', name, enabled });
			this.nodes = this.nodes.map((n) => (n.name === name ? { ...n, enabled } : n));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : 'failed to toggle node';
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
				cluster: 'default',
				name,
				node,
				enabled
			});
			this.storages = this.storages.map((s) =>
				s.name === name && s.node === node ? { ...s, enabled } : s
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : 'failed to toggle storage';
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	async toggleBridge(name: string, enabled: boolean): Promise<void> {
		this.toggling = `bridge:${name}`;
		this.toggleError = null;
		try {
			await post<ToggleResponse>('/api/v1/admin/bridges/toggle', { cluster: 'default', name, enabled });
			this.bridges = this.bridges.map((b) => (b.name === name ? { ...b, enabled } : b));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : 'failed to toggle bridge';
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
				cluster: 'default',
				storage,
				file,
				enabled
			});
			this.isos = this.isos.map((i) =>
				i.storage === storage && i.file === file ? { ...i, enabled } : i
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : 'failed to toggle ISO';
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
