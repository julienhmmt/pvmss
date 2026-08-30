import { getContext, setContext } from 'svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { get, post, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type { VmAction } from './detail.svelte';
import { optimisticStatus } from './detail.svelte';
import { convergeBatch } from './converge';

export type VmStatus = 'running' | 'stopped' | 'paused';
export type VmScope = 'mine' | 'all';
export type VmSortBy = 'vmid' | 'name' | 'node' | 'status' | 'cpu' | 'memory';
export type VmSortDir = 'asc' | 'desc';
export type VmEmptyReason = 'no_vms_owned' | 'no_match';

export interface VmListItem {
	cluster: string;
	clusterDisplayName: string;
	vmid: number;
	name: string;
	node: string;
	status: VmStatus;
	pool: string;
	tags: string[];
	cpuCores: number;
	memoryTotal: number;
}

export interface VmQuota {
	used: number;
	allowed: number;
}

export interface VmListResult {
	items: VmListItem[];
	total: number;
	page: number;
	pageSize: number;
	availableNodes: string[];
	emptyReason?: VmEmptyReason;
	quota?: VmQuota;
}

export interface VmListStoreOptions {
	scope: VmScope;
	/** Raw query string (page.url.search) — the URL is the source of truth on load (FR-007). */
	initialQuery: string;
	/** Pushes query-string changes into the URL without a navigation (goto replaceState). */
	navigate: (queryString: string) => void;
}

/** Result of a per-row power action triggered from the list view. */
export interface RowActionResult {
	ok: boolean;
	error?: string;
}

const DEFAULT_SORT_BY: VmSortBy = 'name';
const DEFAULT_SORT_DIR: VmSortDir = 'asc';
const DEFAULT_PAGE_SIZE = 10;
const SEARCH_DEBOUNCE_MS = 300;

export const SORTABLE_COLUMNS: readonly VmSortBy[] = ['name', 'vmid', 'node', 'status', 'cpu', 'memory'] as const;

/**
 * State and URL synchronization for the single VM-list view (FR-007: search,
 * filter, sort, and page live in the URL so a reload or shared link
 * reproduces the exact view). One store instance per consuming screen;
 * screens differ only in the scope they pass (FR-010).
 */
export class VmListStore {
	readonly scope: VmScope;
	result = $state.raw<VmListResult | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	cluster = $state('');
	search = $state('');
	status = $state<VmStatus | ''>('');
	node = $state('');
	sortBy = $state<VmSortBy>(DEFAULT_SORT_BY);
	sortDir = $state<VmSortDir>(DEFAULT_SORT_DIR);
	page = $state(1);
	pageSize = $state(DEFAULT_PAGE_SIZE);

	#navigate: (queryString: string) => void;
	#searchTimer: ReturnType<typeof setTimeout> | null = null;

	constructor(options: VmListStoreOptions) {
		this.scope = options.scope;
		this.#navigate = options.navigate;
		const params = new SvelteURLSearchParams(options.initialQuery);
		this.cluster = params.get('cluster') ?? '';
		this.search = params.get('search') ?? '';
		this.status = (params.get('status') ?? '') as VmStatus | '';
		this.node = params.get('node') ?? '';
		this.sortBy = (params.get('sortBy') ?? DEFAULT_SORT_BY) as VmSortBy;
		this.sortDir = (params.get('sortDir') ?? DEFAULT_SORT_DIR) as VmSortDir;
		this.page = Math.max(1, Number(params.get('page')) || 1);
		this.pageSize = Number(params.get('pageSize')) || DEFAULT_PAGE_SIZE;
	}

	/** Builds the query string for the current state; only non-default values appear. */
	queryString(): string {
		const params = new SvelteURLSearchParams();
		if (this.cluster !== '') params.set('cluster', this.cluster);
		if (this.search !== '') params.set('search', this.search);
		if (this.status !== '') params.set('status', this.status);
		if (this.node !== '') params.set('node', this.node);
		if (this.sortBy !== DEFAULT_SORT_BY) params.set('sortBy', this.sortBy);
		if (this.sortDir !== DEFAULT_SORT_DIR) params.set('sortDir', this.sortDir);
		if (this.page !== 1) params.set('page', String(this.page));
		if (this.pageSize !== DEFAULT_PAGE_SIZE) params.set('pageSize', String(this.pageSize));
		if (this.scope !== 'mine') params.set('scope', this.scope);
		return params.toString();
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const query = this.queryString();
			this.result = await get<VmListResult>(`/api/v1/vms${query === '' ? '' : `?${query}`}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['vms.list.errorLoading']();
		} finally {
			this.loading = false;
		}
	}

	/** Debounced search input — one field matches name, tag, or ID (FR-002). */
	applySearch(value: string): void {
		this.search = value;
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = setTimeout(() => {
			this.#searchTimer = null;
			this.page = 1;
			this.#syncAndLoad();
		}, SEARCH_DEBOUNCE_MS);
	}

	setCluster(value: string): void {
		this.cluster = value;
		this.page = 1;
		this.#syncAndLoad();
	}

	setStatus(value: VmStatus | ''): void {
		this.status = value;
		this.page = 1;
		this.#syncAndLoad();
	}

	setNode(value: string): void {
		this.node = value;
		this.page = 1;
		this.#syncAndLoad();
	}

	/** Clicking the active column reverses direction; a new column resets to asc. */
	setSort(column: VmSortBy): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
		this.page = 1;
		this.#syncAndLoad();
	}

	setPage(page: number): void {
		this.page = page;
		this.#syncAndLoad();
	}

	setPageSize(pageSize: number): void {
		this.pageSize = pageSize;
		this.page = 1;
		this.#syncAndLoad();
	}

	#syncAndLoad(): void {
		this.#navigate(this.queryString());
		void this.load();
	}

	/**
	 * Triggers a power action on a single VM from the list view, then converges
	 * via the batch live-status endpoint (ADR 0001) — replacing the old `load()`
	 * that overwrote the optimistic flip with a stale projection read.
	 * Returns a result object so the caller can fire the appropriate toast
	 * without coupling the store to the toast queue.
	 */
	async rowAction(cluster: string, vmid: number, action: VmAction): Promise<RowActionResult> {
		const target = optimisticStatus(action);

		// Capture the previous status BEFORE the optimistic flip — the row in
		// `this.result` will be patched to `target` next, so reading it back in
		// the catch block would just return `target` (a no-op revert).
		const row = this.#findRow(cluster, vmid);
		if (row === null) {
			// Row is gone (e.g. deleted concurrently) — nothing to flip or revert.
			return { ok: false, error: m['vms.detail.errorAction']() };
		}
		const previousStatus = row.status;

		// Optimistic flip: patch the row's status immediately.
		this.#patchRowStatus(cluster, vmid, target);

		try {
			await post<{ status: string }>(
				`/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/actions`,
				{ action }
			);
			// Converge: poll batch live status until it matches the optimistic target.
			await convergeBatch({ cluster, vmid }, target, (status) => {
				this.#patchRowStatus(cluster, vmid, status);
			});
			return { ok: true };
		} catch (err) {
			// Revert to the captured previous status.
			this.#patchRowStatus(cluster, vmid, previousStatus);
			return {
				ok: false,
				error: err instanceof ApiRequestError ? err.message : m['vms.detail.errorAction']()
			};
		}
	}

	/** Finds a row by cluster+vmid in the current result set. */
	#findRow(cluster: string, vmid: number): VmListItem | null {
		if (this.result === null) return null;
		return this.result.items.find((r) => r.cluster === cluster && r.vmid === vmid) ?? null;
	}

	/** Patches a single row's status in-place without reloading the list. */
	#patchRowStatus(cluster: string, vmid: number, status: VmStatus): void {
		if (this.result === null) return;
		this.result = {
			...this.result,
			items: this.result.items.map((r) =>
				r.cluster === cluster && r.vmid === vmid ? { ...r, status } : r,
			),
		};
	}
}

const VM_LIST_CONTEXT_KEY = Symbol('vm-list');

/** Called once, by the route that owns this state (constitution VII: no module singletons). */
export function setVmListContext(options: VmListStoreOptions): VmListStore {
	const store = new VmListStore(options);
	setContext(VM_LIST_CONTEXT_KEY, store);
	return store;
}

export function getVmListContext(): VmListStore {
	return getContext<VmListStore>(VM_LIST_CONTEXT_KEY);
}
