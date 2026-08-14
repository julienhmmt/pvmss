import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
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

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.tags = await get<AdminTag[]>('/api/v1/admin/tags?cluster=default');
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
				cluster: 'default', name, color
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
				cluster: 'default', color
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
			await del<{ status: string }>(`/api/v1/admin/tags/${name}?cluster=default`);
			this.tags = this.tags.filter((t) => t.name !== name);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.tags.deleteError']();
			throw err;
		}
	}
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
