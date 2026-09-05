import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
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
	/** True for a stored approval whose node Proxmox no longer reports —
	 *  the row stays listed so the admin can remove it. */
	missing?: boolean;
}

export interface AdminStorage {
	name: string;
	node: string;
	type: string;
	totalBytes: number;
	usedBytes: number;
	enabled: boolean;
	/** True for a synthetic row representing a node that has no storage. */
	noStorage?: boolean;
	/** True for a stored approval whose storage Proxmox no longer reports. */
	missing?: boolean;
}

export interface AdminBridge {
	name: string;
	node: string;
	active: boolean;
	comment: string;
	enabled: boolean;
	/** True for a stored approval whose bridge Proxmox no longer reports. */
	missing?: boolean;
}

export interface AdminISO {
	storage: string;
	node: string;
	file: string;
	sizeBytes: number;
	enabled: boolean;
	/** True for a stored approval whose ISO file Proxmox no longer reports. */
	missing?: boolean;
}

export interface AdminImage {
	storage: string;
	node: string;
	file: string;
	sizeBytes: number;
	enabled: boolean;
	/** True for a stored approval whose image file Proxmox no longer reports. */
	missing?: boolean;
}

export interface AdminTemplate {
	vmid: number;
	node: string;
	name: string;
	cloudInitCapable: boolean;
	diskStorage: string;
	diskSizeGB: number;
	diskBus: string;
	enabled: boolean;
	/** True for a stored approval whose template Proxmox no longer reports —
	 *  the row stays listed so the admin can remove it. */
	missing: boolean;
	/** True when the template's config read failed (issue 03) — the row is
	 *  greyed out and enabling is refused (disabling stays possible). */
	diskUnreadable: boolean;
	/** True when an admin pinned the editable fields (schemaV26). The list
	 *  then shows the stored (overridden) values instead of the discovered
	 *  ones and the drift write-back is skipped. */
	overrideDiscovery: boolean;
}

/** The editable field set of a catalog_templates row, sent on PUT
 *  /api/v1/admin/templates/{cluster}/{vmid} (schemaV26 override). */
export interface AdminTemplatePatch {
	node: string;
	name: string;
	cloudInitCapable: boolean;
	diskStorage: string;
	diskSizeGB: number;
	diskBus: string;
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

interface ImageToggleResponse {
	node: string;
	storage: string;
	file: string;
	enabled: boolean;
}

interface TemplateToggleResponse {
	vmid: number;
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
	cluster = $state('');
	nodes = $state.raw<AdminNode[]>([]);
	storages = $state.raw<AdminStorage[]>([]);
	bridges = $state.raw<AdminBridge[]>([]);
	isos = $state.raw<AdminISO[]>([]);
	images = $state.raw<AdminImage[]>([]);
	templates = $state.raw<AdminTemplate[]>([]);

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

	imageSearch = $state('');
	imageStorageFilter = $state('');
	imageNodeFilter = $state('');
	imageEnabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	imageSortBy: ImageSortColumn = $state('file');
	imageSortDir: 'asc' | 'desc' = $state('asc');

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

	templateSearch = $state('');
	templateStorageFilter = $state('');
	templateNodeFilter = $state('');
	templateSortBy: TemplateSortColumn = $state('vmid');
	templateSortDir: 'asc' | 'desc' = $state('asc');

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

	filteredImages = $derived(
		sortImages(
			this.images.filter((image) => {
				const search = this.imageSearch.toLowerCase();
				if (search && !image.file.toLowerCase().includes(search)) return false;
				if (this.imageStorageFilter && image.storage !== this.imageStorageFilter) return false;
				if (this.imageNodeFilter && image.node !== this.imageNodeFilter) return false;
				if (this.imageEnabledFilter === 'enabled' && !image.enabled) return false;
				if (this.imageEnabledFilter === 'disabled' && image.enabled) return false;
				return true;
			}),
			this.imageSortBy,
			this.imageSortDir
		)
	);

	imageStorageOptions = $derived([...new SvelteSet(this.images.map((i) => i.storage))].sort());
	imageNodeOptions = $derived([...new SvelteSet(this.images.map((i) => i.node))].sort());

	filteredRealStorages = $derived.by(() => {
		const search = this.storageSearch.toLowerCase();
		return this.storages.filter((storage) => {
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
		});
	});

	filteredStorages = $derived.by(() => {
		const search = this.storageSearch.toLowerCase();
		const storageNodes = new SvelteSet(this.storages.map((s) => s.node));
		let selectedNodeNames: string[];
		if (this.storageNodeFilter) {
			selectedNodeNames = this.nodes.some((n) => n.name === this.storageNodeFilter) ? [this.storageNodeFilter] : [];
		} else if (search) {
			selectedNodeNames = this.nodes.filter((n) => n.name.toLowerCase().includes(search)).map((n) => n.name);
		} else {
			selectedNodeNames = this.nodes.map((n) => n.name);
		}

		const placeholders: AdminStorage[] = selectedNodeNames
			.filter((node) => !storageNodes.has(node))
			.map((node) => ({
				name: '',
				node,
				type: '',
				totalBytes: 0,
				usedBytes: 0,
				enabled: false,
				noStorage: true
			}));

		return sortStorages([...this.filteredRealStorages, ...placeholders], this.storageSortBy, this.storageSortDir);
	});

	filteredStorageCount = $derived(this.filteredRealStorages.length);

	storageNodeOptions = $derived([...new SvelteSet(this.nodes.map((n) => n.name))].sort());
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

	filteredTemplates = $derived(
		sortTemplates(
			this.templates.filter((tmpl) => {
				const search = this.templateSearch.toLowerCase();
				if (search && !tmpl.name.toLowerCase().includes(search) && String(tmpl.vmid) !== search) return false;
				if (this.templateStorageFilter && tmpl.diskStorage !== this.templateStorageFilter) return false;
				if (this.templateNodeFilter && tmpl.node !== this.templateNodeFilter) return false;
				return true;
			}),
			this.templateSortBy,
			this.templateSortDir
		)
	);

	templateStorageOptions = $derived(
		[...new SvelteSet(this.templates.map((t) => t.diskStorage).filter((storage) => storage !== ''))].sort()
	);
	templateNodeOptions = $derived([...new SvelteSet(this.templates.map((t) => t.node))].sort());

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

	setImageSort(column: ImageSortColumn): void {
		if (this.imageSortBy === column) {
			this.imageSortDir = this.imageSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.imageSortBy = column;
			this.imageSortDir = 'asc';
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

	resetImageFilters(): void {
		this.imageSearch = '';
		this.imageStorageFilter = '';
		this.imageNodeFilter = '';
		this.imageEnabledFilter = 'all';
		this.imageSortBy = 'file';
		this.imageSortDir = 'asc';
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

	setTemplateSort(column: TemplateSortColumn): void {
		if (this.templateSortBy === column) {
			this.templateSortDir = this.templateSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.templateSortBy = column;
			this.templateSortDir = 'asc';
		}
	}

	resetTemplateFilters(): void {
		this.templateSearch = '';
		this.templateStorageFilter = '';
		this.templateNodeFilter = '';
		this.templateSortBy = 'vmid';
		this.templateSortDir = 'asc';
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

	/** Loads only the template list (plus cluster options). Template discovery
	 *  is N+1 against Proxmox, so it must never ride on loadAll() — the nodes/
	 *  storages/bridges/ISOs pages must not pay for it. */
	async loadTemplates(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		try {
			this.templates = await get<AdminTemplate[]>(`/api/v1/admin/templates?cluster=${encodeURIComponent(this.cluster)}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.catalog.loadError']();
		} finally {
			this.loading = false;
		}
	}

	/** Loads only the image list (plus cluster options). Like loadTemplates(),
	 *  images get their own split load so the nodes/storages/bridges/ISOs
	 *  pages never pay for them. */
	async loadImages(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		try {
			this.images = await get<AdminImage[]>(`/api/v1/admin/images?cluster=${encodeURIComponent(this.cluster)}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.catalog.loadError']();
		} finally {
			this.loading = false;
		}
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

	async toggleImage(node: string, storage: string, file: string, enabled: boolean): Promise<void> {
		this.toggling = `image:${node}:${storage}:${file}`;
		this.toggleError = null;
		try {
			await post<ImageToggleResponse>('/api/v1/admin/images/toggle', {
				cluster: this.cluster,
				node,
				storage,
				file,
				enabled
			});
			this.images = this.images.map((i) =>
				i.node === node && i.storage === storage && i.file === file ? { ...i, enabled } : i
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.images.toggleError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Removes an orphan node approval — offered by the UI on missing rows only. */
	async removeNode(name: string): Promise<void> {
		this.toggling = `node:${name}`;
		this.toggleError = null;
		try {
			await del(`/api/v1/admin/nodes/${encodeURIComponent(this.cluster)}/${encodeURIComponent(name)}`);
			this.nodes = this.nodes.filter((n) => n.name !== name);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.removeNodeError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Removes an orphan storage approval — offered by the UI on missing rows only. */
	async removeStorage(name: string, node: string): Promise<void> {
		this.toggling = `storage:${name}@${node}`;
		this.toggleError = null;
		try {
			await del(`/api/v1/admin/storages/${encodeURIComponent(this.cluster)}/${encodeURIComponent(node)}/${encodeURIComponent(name)}`);
			this.storages = this.storages.filter((s) => !(s.name === name && s.node === node));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.removeStorageError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Removes an orphan bridge approval — offered by the UI on missing rows only. */
	async removeBridge(node: string, name: string): Promise<void> {
		this.toggling = `bridge:${node}/${name}`;
		this.toggleError = null;
		try {
			await del(`/api/v1/admin/bridges/${encodeURIComponent(this.cluster)}/${encodeURIComponent(node)}/${encodeURIComponent(name)}`);
			this.bridges = this.bridges.filter((b) => !(b.node === node && b.name === name));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.removeBridgeError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Removes an orphan ISO approval — offered by the UI on missing rows only. */
	async removeISO(node: string, storage: string, file: string): Promise<void> {
		this.toggling = `iso:${node}:${storage}:${file}`;
		this.toggleError = null;
		try {
			await del(`/api/v1/admin/isos/${encodeURIComponent(this.cluster)}/${encodeURIComponent(node)}/${encodeURIComponent(storage)}/${encodeURIComponent(file)}`);
			this.isos = this.isos.filter((i) => !(i.node === node && i.storage === storage && i.file === file));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.removeIsoError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Removes an orphan image approval — offered by the UI on missing rows only. */
	async removeImage(node: string, storage: string, file: string): Promise<void> {
		this.toggling = `image:${node}:${storage}:${file}`;
		this.toggleError = null;
		try {
			await del(`/api/v1/admin/images/${encodeURIComponent(this.cluster)}/${encodeURIComponent(node)}/${encodeURIComponent(storage)}/${encodeURIComponent(file)}`);
			this.images = this.images.filter((i) => !(i.node === node && i.storage === storage && i.file === file));
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.images.removeError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	async toggleTemplate(vmid: number, enabled: boolean): Promise<void> {
		this.toggling = `template:${vmid}`;
		this.toggleError = null;
		// Optimistic flip (T02): update the row immediately, roll back on
		// failure.
		this.templates = this.templates.map((t) => (t.vmid === vmid ? { ...t, enabled } : t));
		try {
			await post<TemplateToggleResponse>('/api/v1/admin/templates/toggle', {
				cluster: this.cluster,
				vmid,
				enabled
			});
		} catch (err) {
			this.templates = this.templates.map((t) => (t.vmid === vmid ? { ...t, enabled: !enabled } : t));
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.catalog.toggleTemplateError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Removes an approval row (issue 02) — offered by the UI on missing
	 *  (orphaned) rows only; the API deletes any approval. */
	async removeTemplate(vmid: number): Promise<void> {
		this.toggling = `template:${vmid}`;
		this.toggleError = null;
		try {
			await del(`/api/v1/admin/templates/${encodeURIComponent(this.cluster)}/${vmid}`);
			this.templates = this.templates.filter((t) => t.vmid !== vmid);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.templates.removeError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}

	/** Overrides a template's editable fields and pins the row against
	 *  discovery-wins write-back (schemaV26). The cluster comes from the
	 *  store's current cluster selection. */
	async updateTemplate(vmid: number, patch: AdminTemplatePatch): Promise<void> {
		this.toggling = `template:${vmid}`;
		this.toggleError = null;
		try {
			const updated = await put<AdminTemplate>(
				`/api/v1/admin/templates/${encodeURIComponent(this.cluster)}/${vmid}`,
				patch
			);
			// Preserve enabled/missing/diskUnreadable from the existing row;
			// the PUT response only carries the overridden field values.
			this.templates = this.templates.map((t) =>
				t.vmid === vmid
					? { ...t, ...patch, overrideDiscovery: true, enabled: updated.enabled || t.enabled }
					: t
			);
		} catch (err) {
			this.toggleError = err instanceof ApiRequestError ? err.message : m['admin.templates.updateError']();
			throw err;
		} finally {
			this.toggling = null;
		}
	}
}

export type ISOSortColumn = 'file' | 'storage' | 'node' | 'size' | 'enabled';
export type ImageSortColumn = 'file' | 'storage' | 'node' | 'size' | 'enabled';
export type StorageSortColumn = 'name' | 'node' | 'type' | 'usage' | 'enabled';
export type BridgeSortColumn = 'name' | 'node' | 'active' | 'comment' | 'enabled';
export type NodeSortColumn = 'name' | 'status' | 'vmCount' | 'cpuUsage' | 'memoryUsage' | 'enabled';
export type TemplateSortColumn = 'vmid' | 'name' | 'node' | 'disk' | 'cloudInit' | 'enabled';

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

function sortImages(images: AdminImage[], sortBy: ImageSortColumn, dir: 'asc' | 'desc'): AdminImage[] {
	const sorted = [...images].sort((a, b) => {
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

function sortTemplates(templates: AdminTemplate[], sortBy: TemplateSortColumn, dir: 'asc' | 'desc'): AdminTemplate[] {
	const sorted = [...templates].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'vmid':
				cmp = a.vmid - b.vmid || a.name.localeCompare(b.name);
				break;
			case 'name':
				cmp = a.name.localeCompare(b.name) || a.vmid - b.vmid;
				break;
			case 'node':
				cmp = a.node.localeCompare(b.node) || a.vmid - b.vmid;
				break;
			case 'disk':
				cmp = a.diskSizeGB - b.diskSizeGB || a.diskStorage.localeCompare(b.diskStorage) || a.vmid - b.vmid;
				break;
			case 'cloudInit':
				cmp = Number(a.cloudInitCapable) - Number(b.cloudInitCapable) || a.vmid - b.vmid;
				break;
			case 'enabled':
				cmp = Number(a.enabled) - Number(b.enabled) || a.vmid - b.vmid;
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
