import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';

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

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.profiles = await get<AdminProfile[]>('/api/v1/admin/profiles?cluster=default');
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : 'failed to load profiles';
		} finally {
			this.loading = false;
		}
	}

	async create(label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminProfile>('/api/v1/admin/profiles', {
				cluster: 'default', label, cpuCores, memoryMB, diskGB, bus
			});
			this.profiles = [...this.profiles, created];
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to create profile';
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
				cluster: 'default', label, cpuCores, memoryMB, diskGB, bus
			});
			this.profiles = this.profiles.map((p) => (p.id === id ? updated : p));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to update profile';
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async remove(id: string): Promise<void> {
		this.saveError = null;
		try {
			await del<{ status: string }>(`/api/v1/admin/profiles/${id}?cluster=default`);
			this.profiles = this.profiles.filter((p) => p.id !== id);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to delete profile';
			throw err;
		}
	}

	async toggle(id: string, enabled: boolean): Promise<void> {
		this.saveError = null;
		try {
			await post<{ id: string; enabled: boolean }>(`/api/v1/admin/profiles/${id}/toggle`, {
				cluster: 'default', enabled
			});
			this.profiles = this.profiles.map((p) => (p.id === id ? { ...p, enabled } : p));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to toggle profile';
			throw err;
		}
	}
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
