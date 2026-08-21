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

	/** Updates the query and debounces the server call. */
	applySearch(value: string): void {
		this.query = value;
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = setTimeout(() => {
			this.#searchTimer = null;
			void this.load();
		}, SEARCH_DEBOUNCE_MS);
	}

	/** Loads matching VMs from the shared VM list endpoint. */
	async load(): Promise<void> {
		const trimmed = this.query.trim();
		if (trimmed === '') {
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
			this.result = await get<VmListResult>(`/api/v1/vms?${params.toString()}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['search.error']();
		} finally {
			this.loading = false;
		}
	}
}

/** Instantiates a SearchStore and provides it via Svelte context. */
export function setSearchContext(): SearchStore {
	const store = new SearchStore();
	setContext(SEARCH_CONTEXT_KEY, store);
	return store;
}

/** Retrieves the SearchStore from Svelte context. */
export function getSearchContext(): SearchStore {
	return getContext<SearchStore>(SEARCH_CONTEXT_KEY);
}
