import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';

/** AdminDocPage mirrors the adminDocDTO response from GET /api/v1/admin/docs. */
export interface AdminDocPage {
	id: string;
	lang: string;
	title: string;
	category: string;
	bodyMd: string;
	audience: 'user' | 'admin';
	enabled: boolean;
	isSystem: boolean;
	sortOrder: number;
}

/** DocCreateInput is the body sent to POST /api/v1/admin/docs. */
export interface DocCreateInput {
	title: string;
	lang: string;
	category: string;
	bodyMd: string;
	audience: 'user' | 'admin';
}

/** DocUpdateInput is the body sent to PUT /api/v1/admin/docs/{id}/{lang}. */
export interface DocUpdateInput {
	title: string;
	lang: string;
	category: string;
	bodyMd: string;
	audience: 'user' | 'admin';
	enabled: boolean;
	sortOrder: number;
}

/**
 * AdminDocsStore manages the CRUD state for admin-authored documentation
 * pages. API responses are $state.raw — they are API data, not form edits.
 */
export class AdminDocsStore {
	pages = $state.raw<AdminDocPage[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.pages = await get<AdminDocPage[]>('/api/v1/admin/docs');
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : 'failed to load documentation';
		} finally {
			this.loading = false;
		}
	}

	async create(input: DocCreateInput): Promise<AdminDocPage> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminDocPage>('/api/v1/admin/docs', input);
			this.pages = [...this.pages, created];
			return created;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to create page';
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async update(id: string, lang: string, input: DocUpdateInput): Promise<AdminDocPage> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<AdminDocPage>(`/api/v1/admin/docs/${id}/${lang}`, input);
			this.pages = this.pages.map((p) => (p.id === id && p.lang === lang ? updated : p));
			return updated;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to update page';
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async remove(id: string, lang: string): Promise<void> {
		this.saveError = null;
		try {
			await del<{ status: string }>(`/api/v1/admin/docs/${id}/${lang}`);
			this.pages = this.pages.filter((p) => !(p.id === id && p.lang === lang));
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to delete page';
			throw err;
		}
	}

	async toggle(id: string, lang: string, enabled: boolean): Promise<void> {
		this.saveError = null;
		try {
			await post<{ id: string; lang: string; enabled: boolean }>(
				`/api/v1/admin/docs/${id}/${lang}/toggle`,
				{ enabled }
			);
			this.pages = this.pages.map((p) =>
				p.id === id && p.lang === lang ? { ...p, enabled } : p
			);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : 'failed to toggle page';
			throw err;
		}
	}
}

const ADMIN_DOCS_CONTEXT_KEY = Symbol('admin-docs');

export function setAdminDocsContext(): AdminDocsStore {
	const store = new AdminDocsStore();
	setContext(ADMIN_DOCS_CONTEXT_KEY, store);
	return store;
}

export function getAdminDocsContext(): AdminDocsStore {
	return getContext<AdminDocsStore>(ADMIN_DOCS_CONTEXT_KEY);
}
