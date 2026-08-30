import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { setContext, getContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '$lib/paraglide/messages.js';

export interface AdminProfile {
	id: string;
	label: string;
	cpuCores: number;
	memoryMB: number;
	diskGB: number;
	bus: string;
	enabled: boolean;
}

/**
 * AdminProfilesStore manages the CRUD state for VM profiles. API responses are
 * $state.raw — they are API data, not form edits.
 */
export class AdminProfilesStore {
	profiles = $state.raw<AdminProfile[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');

	search = $state('');
	busFilter = $state('');
	enabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	sortBy: 'id' | 'label' | 'cpuCores' | 'memoryMB' | 'diskGB' = $state('label');
	sortDir: 'asc' | 'desc' = $state('asc');

	filteredProfiles = $derived(
		sortProfiles(
			this.profiles.filter((p) => {
				if (this.search) {
					const q = this.search.toLowerCase();
					if (!p.label.toLowerCase().includes(q) && !p.id.toLowerCase().includes(q)) return false;
				}
				if (this.busFilter && p.bus !== this.busFilter) return false;
				if (this.enabledFilter === 'enabled' && !p.enabled) return false;
				if (this.enabledFilter === 'disabled' && p.enabled) return false;
				return true;
			}),
			this.sortBy,
			this.sortDir
		)
	);

	busOptions = $derived([...new SvelteSet(this.profiles.map((p) => p.bus))].sort());

	setSort(column: 'id' | 'label' | 'cpuCores' | 'memoryMB' | 'diskGB'): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
	}

	resetFilters(): void {
		this.search = '';
		this.busFilter = '';
		this.enabledFilter = 'all';
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
			this.error = err instanceof ApiRequestError ? err.message : m['admin.profiles.loadError']();
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
			this.profiles = await get<AdminProfile[]>(`/api/v1/admin/profiles?cluster=${encodeURIComponent(this.cluster)}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.profiles.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async create(label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminProfile>('/api/v1/admin/profiles', {
				cluster: this.cluster, label, cpuCores, memoryMB, diskGB, bus
			});
			this.profiles = [...this.profiles, created];
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.createError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async update(id: string, label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<AdminProfile>(`/api/v1/admin/profiles/${id}`, {
				cluster: this.cluster, label, cpuCores, memoryMB, diskGB, bus
			});
			this.profiles = this.profiles.map((p) => (p.id === id ? updated : p));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.updateError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async remove(id: string): Promise<void> {
		this.saveError = null;
		try {
			await del<{ status: string }>(`/api/v1/admin/profiles/${id}?cluster=${encodeURIComponent(this.cluster)}`);
			this.profiles = this.profiles.filter((p) => p.id !== id);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.deleteError']();
			throw err;
		}
	}

	async toggle(id: string, enabled: boolean): Promise<void> {
		this.saveError = null;
		try {
			await post<{ id: string; enabled: boolean }>(`/api/v1/admin/profiles/${id}/toggle`, {
				cluster: this.cluster, enabled
			});
			this.profiles = this.profiles.map((p) => (p.id === id ? { ...p, enabled } : p));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.toggleError']();
			throw err;
		}
	}
}

type ProfileSortColumn = 'id' | 'label' | 'cpuCores' | 'memoryMB' | 'diskGB';

function sortProfiles(profiles: AdminProfile[], sortBy: ProfileSortColumn, dir: 'asc' | 'desc'): AdminProfile[] {
	const sorted = [...profiles].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'id':
				cmp = a.id.localeCompare(b.id);
				break;
			case 'label':
				cmp = a.label.localeCompare(b.label) || a.id.localeCompare(b.id);
				break;
			case 'cpuCores':
				cmp = a.cpuCores - b.cpuCores || a.label.localeCompare(b.label);
				break;
			case 'memoryMB':
				cmp = a.memoryMB - b.memoryMB || a.label.localeCompare(b.label);
				break;
			case 'diskGB':
				cmp = a.diskGB - b.diskGB || a.label.localeCompare(b.label);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}

const PROFILES_CONTEXT_KEY = Symbol('admin-profiles');

export function setAdminProfilesContext(): AdminProfilesStore {
	const store = new AdminProfilesStore();
	setContext(PROFILES_CONTEXT_KEY, store);
	return store;
}

export function getAdminProfilesContext(): AdminProfilesStore {
	return getContext<AdminProfilesStore>(PROFILES_CONTEXT_KEY);
}
