import { getContext, setContext } from 'svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { get, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type { VmListResult } from '$lib/features/vms/list.svelte';

const DEFAULT_PAGE_SIZE = 15;
const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_SCOPE = 'all';
const DEFAULT_SORT_BY = 'name';
const DEFAULT_SORT_DIR = 'asc';

const SEARCH_CONTEXT_KEY = Symbol('search');

/**
 * Owns the query, loading, error and result state for the global VM search
 * page. Queries are debounced and sent to the existing /api/v1/vms endpoint
 * with the search parameter, which already matches by name, tag or VM ID.
 */
export class SearchStore {
	query = $state('');
	loading = $state(false);
	error = $state<string | null>(null);
	result = $state<VmListResult | null>(null);

	#searchTimer: ReturnType<typeof setTimeout> | null = null;
	#navigate: (queryString: string) => void;
	#committedQuery: string = '';
	#requestId: number = 0;

	/** initialQuery is the page's URL search (`?q=`); navigate writes `q` back. */
	constructor(options: { initialQuery?: string; navigate?: (queryString: string) => void } = {}) {
		this.query = new SvelteURLSearchParams(options.initialQuery ?? '').get('q') ?? '';
		this.#navigate = options.navigate ?? (() => {});
		this.#committedQuery = this.query;
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
				this.#navigate(trimmed === '' ? '' : new SvelteURLSearchParams({ q: trimmed }).toString());
			}
			void this.load();
		}, SEARCH_DEBOUNCE_MS);
	}

	restoreQuery(queryString: string): void {
		const query: string = new SvelteURLSearchParams(queryString).get('q') ?? '';
		if (query === this.#committedQuery) return;
		this.dispose();
		this.#committedQuery = query;
		this.query = query;
		void this.load();
	}

	dispose(): void {
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = null;
		this.#requestId += 1;
	}

	/** Loads matching VMs from the shared VM list endpoint. */
	async load(): Promise<void> {
		const requestId: number = ++this.#requestId;
		const trimmed: string = this.query.trim();
		if (trimmed === '') {
			this.loading = false;
			this.result = null;
			this.error = null;
			return;
		}

		this.loading = true;
		this.error = null;
		try {
			const params = new SvelteURLSearchParams();
			params.set('search', trimmed);
			params.set('scope', SEARCH_SCOPE);
			params.set('sortBy', DEFAULT_SORT_BY);
			params.set('sortDir', DEFAULT_SORT_DIR);
			params.set('pageSize', String(DEFAULT_PAGE_SIZE));
			const result: VmListResult = await get<VmListResult>(`/api/v1/vms?${params.toString()}`);
			if (requestId === this.#requestId) this.result = result;
		} catch (err) {
			if (requestId === this.#requestId) this.error = err instanceof ApiRequestError ? err.message : m['search.error']();
		} finally {
			if (requestId === this.#requestId) this.loading = false;
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
