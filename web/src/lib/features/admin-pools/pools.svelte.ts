import { del, get, post, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { getContext, setContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface AdminPool {
	name: string;
	comment: string;
	total: number;
	running: number;
	stopped: number;
	managed: boolean;
}

/** CreatedPoolCredentials holds the server-generated credentials returned once. */
export interface CreatedPoolCredentials {
	name: string;
	username: string;
	password: string;
	comment: string;
	managed: boolean;
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
	createdCredentials = $state.raw<CreatedPoolCredentials | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');
	#searchTimer: ReturnType<typeof setTimeout> | null = null;

	/** Resolves the real cluster name so requests never send the literal "default"
	 *  against a deployment whose cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			const first = this.clusterOptions[0];
			if (first && (this.clusterOptions.length === 1 || !this.clusterOptions.some((option) => option.name === this.cluster))) {
				this.cluster = first.name;
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.pools.loadError']();
		}
	}

	setCluster(value: string): void {
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		if (this.clusterOptions.length === 0) {
			await this.loadClusters();
		}
		this.loading = true;
		this.error = null;
		try {
			const query = this.searchTerm ? `&search=${encodeURIComponent(this.searchTerm)}` : '';
			this.pools = await get<AdminPool[]>(`/api/v1/admin/pools?cluster=${encodeURIComponent(this.cluster)}${query}`);
		} catch (error) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.pools.loadError']();
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

	async create(name: string, comment: string = ''): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.announce = null;
		this.createdCredentials = null;
		try {
			const created = await post<CreatedPoolCredentials>(`/api/v1/admin/pools?cluster=${encodeURIComponent(this.cluster)}`, {
				name,
				comment
			});
			const needle = this.searchTerm.toLowerCase();
			if (needle === '' || created.name.toLowerCase().includes(needle)) {
				this.pools = [...this.pools, {
					name: created.name,
					comment: created.comment,
					total: 0,
					running: 0,
					stopped: 0,
					managed: created.managed
				}];
			}
			this.createdCredentials = created;
			this.announce = m['admin.pools.created']({ name: created.name });
		} catch (error) {
			this.saveError = error instanceof ApiRequestError ? error.message : m['admin.pools.createError']();
			throw error;
		} finally {
			this.saving = false;
		}
	}

	dismissCredentials(): void {
		this.createdCredentials = null;
	}

	async remove(name: string): Promise<void> {
		this.deleting = name;
		this.deleteError = null;
		this.announce = null;
		try {
			const result = await del<DeletePoolResponse>(`/api/v1/admin/pools/${encodeURIComponent(name)}?cluster=${encodeURIComponent(this.cluster)}`);
			this.pools = this.pools.filter((pool) => pool.name !== name);
			this.announce = result.userDeleted
				? m['admin.pools.deleted']({ name })
				: m['admin.pools.deletedUserFailed']({ name });
		} catch (error) {
			this.deleteError = error instanceof ApiRequestError ? error.message : m['admin.pools.deleteError']();
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
