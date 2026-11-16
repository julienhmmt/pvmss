import { get, post, put, del, ApiRequestError } from '#lib/shared/api/client.js';
import {
	fetchClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from '#lib/shared/clusters.js';
import { setContext, getContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '#lib/paraglide/messages.js';

export interface AdminProfile {
	id: string;
	label: string;
	sockets: number;
	cpuCores: number;
	memoryMB: number;
	diskGB: number;
	bus: string;
	enabled: boolean;
	description: string;
}

/** vCPU total a VM gets from this profile: sockets x cores (sockets default to 1). */
/** Export file kind tag and version; bump the version on a breaking change. */
export const PROFILE_EXPORT_KIND = 'pvmss-profiles';
export const PROFILE_EXPORT_VERSION = 1;

export type ProfileExportItem = Omit<AdminProfile, 'id'> & { id?: string };

export interface ProfileImportOutcome {
	cluster: string;
	id: string;
	label: string;
	result: 'created' | 'updated' | 'skipped' | 'invalid';
	error?: string;
}

/** Serialises profiles (in the given order) to the export file content. */
export function buildProfileExport(profiles: AdminProfile[]): string {
	return JSON.stringify({ kind: PROFILE_EXPORT_KIND, version: PROFILE_EXPORT_VERSION, profiles }, null, 2);
}

/** Parses an export file. Field values are validated by the server on import. */
export function parseProfileExport(text: string): ProfileExportItem[] {
	const data: unknown = JSON.parse(text);
	if (
		typeof data !== 'object' || data === null ||
		(data as { kind?: unknown }).kind !== PROFILE_EXPORT_KIND ||
		(data as { version?: unknown }).version !== PROFILE_EXPORT_VERSION ||
		!Array.isArray((data as { profiles?: unknown }).profiles)
	) {
		throw new Error(m['admin.profiles.importInvalidFile']());
	}
	const items = (data as { profiles: unknown[] }).profiles;
	if (!items.every((p) => typeof p === 'object' && p !== null && typeof (p as { label?: unknown }).label === 'string')) {
		throw new Error(m['admin.profiles.importInvalidFile']());
	}
	return items as ProfileExportItem[];
}

export function vcpuTotal(p: Pick<AdminProfile, 'sockets' | 'cpuCores'>): number {
	return (p.sockets ?? 1) * p.cpuCores;
}

/**
 * AdminProfilesStore manages the CRUD state for VM profiles. API responses are
 * $state.raw - they are API data, not form edits.
 */
export class AdminProfilesStore {
	profiles = $state.raw<AdminProfile[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('');

	search = $state('');
	busFilter = $state('');
	enabledFilter: 'all' | 'enabled' | 'disabled' = $state('all');
	sortBy: ProfileSortColumn = $state('position');
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
	/** Latest-wins guard for load: a cluster switch must not show the previous cluster's profiles. */
	#loadRequestId = 0;

	/** Ids checked for export, scoped to the loaded cluster. */
	selected = new SvelteSet<string>();

	/** Reordering only makes sense on the unfiltered list in stored order. */
	canReorder = $derived(
		this.sortBy === 'position' && this.sortDir === 'asc' && !this.search && !this.busFilter && this.enabledFilter === 'all'
	);

	setSort(column: ProfileSortColumn): void {
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
		this.sortBy = 'position';
		this.sortDir = 'asc';
	}

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			if (this.clusterOptions.length > 0) {
				this.cluster = resolveInitialAdminCluster(this.clusterOptions, this.cluster);
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.profiles.loadError']();
		}
	}

	setCluster(value: string): void {
		saveAdminCluster(value);
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		const requestId = ++this.#loadRequestId;
		await this.loadClusters();
		if (requestId !== this.#loadRequestId) return;
		this.loading = true;
		this.error = null;
		try {
			const profiles = await get<AdminProfile[]>(`/api/v1/admin/profiles?cluster=${encodeURIComponent(this.cluster)}`);
			if (requestId === this.#loadRequestId) {
				this.profiles = profiles;
				this.selected.clear();
			}
		} catch (err) {
			if (requestId === this.#loadRequestId) {
				this.error = err instanceof ApiRequestError ? err.message : m['admin.profiles.loadError']();
			}
		} finally {
			if (requestId === this.#loadRequestId) this.loading = false;
		}
	}

	async create(label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string, sockets = 1, description = ''): Promise<boolean> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminProfile>('/api/v1/admin/profiles', {
				cluster: this.cluster, label, sockets, cpuCores, memoryMB, diskGB, bus, description
			});
			this.profiles = [...this.profiles, created];
			return true;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.createError']();
			return false;
		} finally {
			this.saving = false;
		}
	}

	async update(id: string, label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string, sockets = 1, description = ''): Promise<boolean> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<AdminProfile>(`/api/v1/admin/profiles/${id}`, {
				cluster: this.cluster, label, sockets, cpuCores, memoryMB, diskGB, bus, description
			});
			this.profiles = this.profiles.map((p) => (p.id === id ? updated : p));
			return true;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.updateError']();
			return false;
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

	/** Moves a profile one step up (-1) or down (+1) and saves the new order;
	 *  the user-side picker follows the same order. Reverts on failure. */
	async move(id: string, delta: -1 | 1): Promise<void> {
		const from = this.profiles.findIndex((p) => p.id === id);
		const to = from + delta;
		if (from < 0 || to < 0 || to >= this.profiles.length) return;
		const previous = this.profiles;
		const next = [...previous];
		next.splice(to, 0, ...next.splice(from, 1));
		this.profiles = next;
		this.saveError = null;
		try {
			await put<{ status: string }>('/api/v1/admin/profiles/order', {
				cluster: this.cluster,
				ids: next.map((p) => p.id)
			});
		} catch (err) {
			this.profiles = previous;
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.reorderError']();
		}
	}

	/** Selected profiles (or all when none is selected), in stored order. */
	exportContent(): string {
		const chosen = this.selected.size > 0 ? this.profiles.filter((p) => this.selected.has(p.id)) : this.profiles;
		return buildProfileExport(chosen);
	}

	async importProfiles(clusters: string[], overwrite: boolean, profiles: ProfileExportItem[]): Promise<ProfileImportOutcome[] | null> {
		this.saving = true;
		this.saveError = null;
		try {
			const outcomes = await post<ProfileImportOutcome[]>('/api/v1/admin/profiles/import', {
				clusters,
				overwrite,
				profiles: profiles.map(({ label, sockets, cpuCores, memoryMB, diskGB, bus, enabled, description }) => ({
					label, sockets, cpuCores, memoryMB, diskGB, bus, enabled, description
				}))
			});
			await this.load();
			return outcomes;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.profiles.importError']();
			return null;
		} finally {
			this.saving = false;
		}
	}
}

export type ProfileSortColumn = 'position' | 'id' | 'label' | 'cpuCores' | 'memoryMB' | 'diskGB';

function sortProfiles(profiles: AdminProfile[], sortBy: ProfileSortColumn, dir: 'asc' | 'desc'): AdminProfile[] {
	const sorted = [...profiles].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'position':
				return 0; // the server returns profiles in stored order; sort is stable
			case 'id':
				cmp = a.id.localeCompare(b.id);
				break;
			case 'label':
				cmp = a.label.localeCompare(b.label) || a.id.localeCompare(b.id);
				break;
			case 'cpuCores':
				cmp = vcpuTotal(a) - vcpuTotal(b) || a.label.localeCompare(b.label);
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
