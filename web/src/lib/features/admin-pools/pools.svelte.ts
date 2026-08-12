import { del, get, post, ApiRequestError } from '$lib/shared/api/client';
import { getContext, setContext } from 'svelte';

export interface AdminPool {
	name: string;
	comment: string;
	total: number;
	running: number;
	stopped: number;
}

interface DeletePoolResponse {
	status: string;
	userDeleted: boolean;
}

const ADMIN_POOLS_CONTEXT_KEY = Symbol('admin-pools');

/** Manages admin pool list, provisioning, filtering, and deletion state. */
const SEARCH_DEBOUNCE_MS = 300;

export class PoolsStore {
	pools = $state.raw<AdminPool[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	deleting = $state.raw<string | null>(null);
	deleteError = $state.raw<string | null>(null);
	searchTerm = $state.raw('');
	announce = $state.raw<string | null>(null);
	#searchTimer: ReturnType<typeof setTimeout> | null = null;

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const query = this.searchTerm ? `&search=${encodeURIComponent(this.searchTerm)}` : '';
			this.pools = await get<AdminPool[]>(`/api/v1/admin/pools?cluster=default${query}`);
		} catch (error) {
			this.error = error instanceof ApiRequestError ? error.message : 'failed to load admin pools';
		} finally {
			this.loading = false;
		}
	}

	applySearch(value: string): void {
		this.searchTerm = value;
		if (this.#searchTimer !== null) clearTimeout(this.#searchTimer);
		this.#searchTimer = setTimeout(() => {
			this.#searchTimer = null;
			void this.load();
		}, SEARCH_DEBOUNCE_MS);
	}

	async create(name: string, password: string, comment: string = ''): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.announce = null;
		try {
			const created = await post<AdminPool>('/api/v1/admin/pools', {
				name,
				comment,
				password
			});
			const needle = this.searchTerm.toLowerCase();
			if (needle === '' || created.name.toLowerCase().includes(needle)) {
				this.pools = [...this.pools, created];
			}
			this.announce = `Pool ${created.name} created`;
		} catch (error) {
			this.saveError = error instanceof ApiRequestError ? error.message : 'failed to create pool';
			throw error;
		} finally {
			this.saving = false;
		}
	}

	async remove(name: string): Promise<void> {
		this.deleting = name;
		this.deleteError = null;
		this.announce = null;
		try {
			const result = await del<DeletePoolResponse>(`/api/v1/admin/pools/${encodeURIComponent(name)}?cluster=default`);
			this.pools = this.pools.filter((pool) => pool.name !== name);
			this.announce = result.userDeleted
				? `Pool ${name} deleted`
				: `Pool ${name} deleted, but its user could not be deleted`;
		} catch (error) {
			this.deleteError = error instanceof ApiRequestError ? error.message : 'failed to delete pool';
			throw error;
		} finally {
			this.deleting = null;
		}
	}
}

export function setAdminPoolsContext(): PoolsStore {
	const store = new PoolsStore();
	setContext(ADMIN_POOLS_CONTEXT_KEY, store);
	return store;
}

export function getAdminPoolsContext(): PoolsStore {
	return getContext<PoolsStore>(ADMIN_POOLS_CONTEXT_KEY);
}
