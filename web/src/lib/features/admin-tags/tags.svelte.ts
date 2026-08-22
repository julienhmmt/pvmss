import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface AdminTag {
	name: string;
	color: string;
	vmCount: number;
	protected: boolean;
}

/**
 * AdminTagsStore manages the CRUD state for catalog tags. API responses are
 * $state.raw — they are API data, not form edits.
 */
export class AdminTagsStore {
	tags = $state.raw<AdminTag[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');

	search = $state('');
	protectedFilter: 'all' | 'protected' | 'unprotected' = $state('all');
	sortBy: 'name' | 'vmCount' = $state('name');
	sortDir: 'asc' | 'desc' = $state('asc');

	filteredTags = $derived(
		sortTags(
			this.tags.filter((t) => {
				if (this.search && !t.name.toLowerCase().includes(this.search.toLowerCase())) return false;
				if (this.protectedFilter === 'protected' && !t.protected) return false;
				if (this.protectedFilter === 'unprotected' && t.protected) return false;
				return true;
			}),
			this.sortBy,
			this.sortDir
		)
	);

	setSort(column: 'name' | 'vmCount'): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
	}

	resetFilters(): void {
		this.search = '';
		this.protectedFilter = 'all';
	}

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			const first = this.clusterOptions[0];
			if (first && (this.clusterOptions.length === 1 || !this.clusterOptions.some((option) => option.name === this.cluster))) {
				this.cluster = first.name;
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.tags.loadError']();
		}
	}

	setCluster(value: string): void {
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		try {
			this.tags = await get<AdminTag[]>(`/api/v1/admin/tags?cluster=${encodeURIComponent(this.cluster)}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.tags.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async create(name: string, color: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminTag>('/api/v1/admin/tags', {
				cluster: this.cluster, name, color
			});
			this.tags = [...this.tags, created];
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.tags.createError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async updateColor(name: string, color: string): Promise<void> {
		this.saveError = null;
		try {
			const updated = await put<AdminTag>(`/api/v1/admin/tags/${name}/color`, {
				cluster: this.cluster, color
			});
			this.tags = this.tags.map((t) => (t.name === name ? { ...updated, vmCount: t.vmCount } : t));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.tags.updateColorError']();
			throw err;
		}
	}

	async remove(name: string): Promise<void> {
		this.saveError = null;
		try {
			await del<{ status: string }>(`/api/v1/admin/tags/${name}?cluster=${encodeURIComponent(this.cluster)}`);
			this.tags = this.tags.filter((t) => t.name !== name);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.tags.deleteError']();
			throw err;
		}
	}
}

type TagSortColumn = 'name' | 'vmCount';

function sortTags(tags: AdminTag[], sortBy: TagSortColumn, dir: 'asc' | 'desc'): AdminTag[] {
	const sorted = [...tags].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'name':
				cmp = a.name.localeCompare(b.name);
				break;
			case 'vmCount':
				cmp = a.vmCount - b.vmCount || a.name.localeCompare(b.name);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}

const TAGS_CONTEXT_KEY = Symbol('admin-tags');

export function setAdminTagsContext(): AdminTagsStore {
	const store = new AdminTagsStore();
	setContext(TAGS_CONTEXT_KEY, store);
	return store;
}

export function getAdminTagsContext(): AdminTagsStore {
	return getContext<AdminTagsStore>(TAGS_CONTEXT_KEY);
}
