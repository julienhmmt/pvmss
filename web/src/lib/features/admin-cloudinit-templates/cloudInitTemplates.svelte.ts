import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface AdminCloudInitTemplate {
	id: string;
	label: string;
	content: string;
	enabled: boolean;
}

/**
 * AdminCloudInitTemplatesStore manages the CRUD state for cloud-init
 * templates. API responses are $state.raw — they are API data, not form edits.
 */
export class AdminCloudInitTemplatesStore {
	templates = $state.raw<AdminCloudInitTemplate[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.templates = await get<AdminCloudInitTemplate[]>(
				'/api/v1/admin/cloudinit-templates?cluster=default'
			);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async create(label: string, content: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminCloudInitTemplate>('/api/v1/admin/cloudinit-templates', {
				cluster: 'default',
				label,
				content
			});
			this.templates = [...this.templates, created];
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.createError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async update(id: string, label: string, content: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<AdminCloudInitTemplate>(
				`/api/v1/admin/cloudinit-templates/${id}`,
				{ cluster: 'default', label, content }
			);
			this.templates = this.templates.map((t) => (t.id === id ? updated : t));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.updateError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async remove(id: string): Promise<void> {
		this.saveError = null;
		try {
			await del<{ status: string }>(`/api/v1/admin/cloudinit-templates/${id}?cluster=default`);
			this.templates = this.templates.filter((t) => t.id !== id);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.deleteError']();
			throw err;
		}
	}

	async toggle(id: string, enabled: boolean): Promise<void> {
		this.saveError = null;
		try {
			await post<{ id: string; enabled: boolean }>(
				`/api/v1/admin/cloudinit-templates/${id}/toggle`,
				{ cluster: 'default', enabled }
			);
			this.templates = this.templates.map((t) => (t.id === id ? { ...t, enabled } : t));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.toggleError']();
			throw err;
		}
	}
}

const CLOUDINIT_TEMPLATES_CONTEXT_KEY = Symbol('admin-cloudinit-templates');

export function setAdminCloudInitTemplatesContext(): AdminCloudInitTemplatesStore {
	const store = new AdminCloudInitTemplatesStore();
	setContext(CLOUDINIT_TEMPLATES_CONTEXT_KEY, store);
	return store;
}

export function getAdminCloudInitTemplatesContext(): AdminCloudInitTemplatesStore {
	return getContext<AdminCloudInitTemplatesStore>(CLOUDINIT_TEMPLATES_CONTEXT_KEY);
}
