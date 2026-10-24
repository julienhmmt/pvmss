import { getContext, setContext } from 'svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { get, ApiRequestError } from '#lib/shared/api/client.js';
import { fetchClusterOptions, type ClusterOption } from '#lib/shared/clusters.js';
import { m } from '#lib/paraglide/messages.js';
import type { VmListResult, VmStatus } from '#lib/features/vms/list.svelte.js';

const DEFAULT_PAGE_SIZE = 15;
const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_SCOPE = 'all';
const DEFAULT_SORT_BY = 'name';
const DEFAULT_SORT_DIR = 'asc';

const SEARCH_CONTEXT_KEY = Symbol('search');

const VALID_STATUSES: readonly VmStatus[] = ['running', 'stopped', 'paused'];

function parseStatus(value: string | null): '' | VmStatus {
	return VALID_STATUSES.includes(value as VmStatus) ? (value as VmStatus) : '';
}

/**
 * Owns the query, filter, loading, error and result state for the global VM
 * search page. Queries are debounced and sent to the existing /api/v1/vms
 * endpoint with the search parameter, which already matches by name, tag or
 * VM ID; the cluster and status filters are passed through as-is and synced
 * to the URL query string (`?q=&cluster=&status=`).
 */
export class SearchStore {
	query = $state('');
	cluster = $state('');
	status = $state<'' | VmStatus>('');
	clusterOptions = $state.raw<ClusterOption[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);
	result = $state<VmListResult | null>(null);
	loadingMore = $state(false);

	#searchTimer: ReturnType<typeof setTimeout> | null = null;
	#navigate: (queryString: string) => void;
	#committedQuery: string = '';
	#committedCluster: string = '';
	#committedStatus: '' | VmStatus = '';
	#requestId: number = 0;

	/** initialQuery is the page's URL search (`?q=&cluster=&status=`);
	 *  navigate writes the committed state back. */
	constructor(options: { initialQuery?: string; navigate?: (queryString: string) => void } = {}) {
		const params = new SvelteURLSearchParams(options.initialQuery ?? '');
		this.query = params.get('q') ?? '';
		this.cluster = params.get('cluster') ?? '';
		this.status = parseStatus(params.get('status'));
		this.#navigate = options.navigate ?? (() => {});
		this.#committedQuery = this.query;
		this.#committedCluster = this.cluster;
		this.#committedStatus = this.status;
	}

	/** Fetches the cluster list for the filter selector (non-fatal on error). */
	async init(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
		} catch {
			this.clusterOptions = [];
		}
	}

	/** Builds the URL query string for the committed query + filters. */
	#buildQueryString(): string {
		const params = new SvelteURLSearchParams();
		if (this.#committedQuery !== '') params.set('q', this.#committedQuery);
		if (this.cluster !== '') params.set('cluster', this.cluster);
		if (this.status !== '') params.set('status', this.status);
		return params.toString();
	}

	/** Commits the current query + filters, syncs the URL and reloads. */
	#commitFilters(): void {
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = null;
		this.#committedQuery = this.query.trim();
		this.#committedCluster = this.cluster;
		this.#committedStatus = this.status;
		this.#navigate(this.#buildQueryString());
		void this.load();
	}

	/** Updates the cluster filter and reloads immediately. */
	setCluster(value: string): void {
		if (value === this.cluster) return;
		this.cluster = value;
		this.#commitFilters();
	}

	/** Updates the status filter and reloads immediately. */
	setStatus(value: '' | VmStatus): void {
		if (value === this.status) return;
		this.status = value;
		this.#commitFilters();
	}

	/** Updates the query and debounces the server call. */
	applySearch(value: string): void {
		this.query = value;
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = setTimeout(() => {
			this.#searchTimer = null;
			const trimmed: string = this.query.trim();
			if (trimmed !== this.#committedQuery) {
				this.#committedQuery = trimmed;
				this.#navigate(this.#buildQueryString());
			}
			void this.load();
		}, SEARCH_DEBOUNCE_MS);
	}

	restoreQuery(queryString: string): void {
		const params = new SvelteURLSearchParams(queryString);
		const query: string = params.get('q') ?? '';
		const cluster: string = params.get('cluster') ?? '';
		const status: '' | VmStatus = parseStatus(params.get('status'));
		if (query === this.#committedQuery && cluster === this.#committedCluster && status === this.#committedStatus) return;
		this.dispose();
		this.#committedQuery = query;
		this.#committedCluster = cluster;
		this.#committedStatus = status;
		this.query = query;
		this.cluster = cluster;
		this.status = status;
		void this.load();
	}

	dispose(): void {
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = null;
		this.#requestId += 1;
	}

	/** True when the server holds more matches than the rows on screen. */
	get hasMore(): boolean {
		return this.result !== null && this.result.items.length < this.result.total;
	}

	/** One page of matching VMs from the shared VM list endpoint. */
	#fetchPage(page: number): Promise<VmListResult> {
		const params = new SvelteURLSearchParams();
		const trimmed: string = this.query.trim();
		if (trimmed !== '') params.set('search', trimmed);
		if (this.cluster !== '') params.set('cluster', this.cluster);
		if (this.status !== '') params.set('status', this.status);
		params.set('scope', SEARCH_SCOPE);
		params.set('sortBy', DEFAULT_SORT_BY);
		params.set('sortDir', DEFAULT_SORT_DIR);
		params.set('page', String(page));
		params.set('pageSize', String(DEFAULT_PAGE_SIZE));
		return get<VmListResult>(`/api/v1/vms?${params.toString()}`);
	}

	/** Loads the first page of matches, replacing any previous result. */
	async load(): Promise<void> {
		const requestId: number = ++this.#requestId;
		this.loadingMore = false;
		if (this.query.trim() === '' && this.cluster === '' && this.status === '') {
			this.loading = false;
			this.result = null;
			this.error = null;
			return;
		}

		this.loading = true;
		this.error = null;
		try {
			const result: VmListResult = await this.#fetchPage(1);
			if (requestId === this.#requestId) this.result = result;
		} catch (err) {
			if (requestId === this.#requestId) this.error = err instanceof ApiRequestError ? err.message : m['search.error']();
		} finally {
			if (requestId === this.#requestId) this.loading = false;
		}
	}

	/** Appends the next page of matches to the current result. */
	async loadMore(): Promise<void> {
		const current = this.result;
		if (current === null || !this.hasMore || this.loadingMore) return;
		const requestId: number = this.#requestId;
		this.loadingMore = true;
		try {
			const next: VmListResult = await this.#fetchPage(current.page + 1);
			if (requestId === this.#requestId) this.result = { ...next, items: [...current.items, ...next.items] };
		} catch (err) {
			if (requestId === this.#requestId) this.error = err instanceof ApiRequestError ? err.message : m['search.error']();
		} finally {
			if (requestId === this.#requestId) this.loadingMore = false;
		}
	}
}

/** Instantiates a SearchStore and provides it via Svelte context. */
export function setSearchContext(options: ConstructorParameters<typeof SearchStore>[0] = {}): SearchStore {
	const store = new SearchStore(options);
	setContext(SEARCH_CONTEXT_KEY, store);
	return store;
}

/** Retrieves the SearchStore from Svelte context. */
export function getSearchContext(): SearchStore {
	return getContext<SearchStore>(SEARCH_CONTEXT_KEY);
}
