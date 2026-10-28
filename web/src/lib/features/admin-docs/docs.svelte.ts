import { get, post, put, del, ApiRequestError } from '#lib/shared/api/client.js';
import { setContext, getContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '#lib/paraglide/messages.js';

export type DocSortColumn = 'order' | 'title' | 'id' | 'category' | 'lang';

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
 * pages. API responses are $state.raw - they are API data, not form edits.
 */
export class AdminDocsStore {
	pages = $state.raw<AdminDocPage[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);

	search = $state('');
	categoryFilter = $state('');
	langFilter = $state('');
	audienceFilter: 'all' | 'user' | 'admin' = $state('all');
	sortBy: DocSortColumn = $state('order');
	sortDir: 'asc' | 'desc' = $state('asc');

	filteredPages = $derived(
		sortDocs(
			this.pages.filter((p) => {
				if (this.search) {
					const q = this.search.toLowerCase();
					if (!p.title.toLowerCase().includes(q) && !p.id.toLowerCase().includes(q)) return false;
				}
				if (this.categoryFilter && p.category !== this.categoryFilter) return false;
				if (this.langFilter && p.lang !== this.langFilter) return false;
				if (this.audienceFilter !== 'all' && p.audience !== this.audienceFilter) return false;
				return true;
			}),
			this.sortBy,
			this.sortDir
		)
	);

	categoryOptions = $derived([...new SvelteSet(this.pages.map((p) => p.category))].sort());
	langOptions = $derived([...new SvelteSet(this.pages.map((p) => p.lang))].sort());

	setSort(column: DocSortColumn): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
	}

	resetFilters(): void {
		this.search = '';
		this.categoryFilter = '';
		this.langFilter = '';
		this.audienceFilter = 'all';
		this.sortBy = 'order';
		this.sortDir = 'asc';
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.pages = await get<AdminDocPage[]>('/api/v1/admin/docs');
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.docs.loadError']();
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
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.docs.createError']();
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
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.docs.updateError']();
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
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.docs.deleteError']();
			throw err;
		}
	}

	/** orderedIds lists each page id once, in the saved order (sortOrder, then title). */
	orderedIds(): string[] {
		return [...new SvelteSet(sortDocs(this.pages, 'order', 'asc').map((p) => p.id))];
	}

	/** moveTo puts page `id` at the position currently held by `targetId` and saves the full order. */
	async moveTo(id: string, targetId: string): Promise<void> {
		const ids = this.orderedIds();
		const from = ids.indexOf(id);
		const to = ids.indexOf(targetId);
		if (from < 0 || to < 0 || from === to) return;
		const next = ids.filter((x) => x !== id);
		next.splice(to, 0, id);
		await this.saveOrder(next);
	}

	/** moveBy shifts page `id` by `delta` positions (-1 up, +1 down). */
	async moveBy(id: string, delta: number): Promise<void> {
		const ids = this.orderedIds();
		const target = ids[ids.indexOf(id) + delta];
		if (target !== undefined) await this.moveTo(id, target);
	}

	private async saveOrder(ids: string[]): Promise<void> {
		const previous = this.pages;
		this.saveError = null;
		this.pages = previous.map((p) => ({ ...p, sortOrder: ids.indexOf(p.id) }));
		try {
			await put<{ status: string }>('/api/v1/admin/docs/order', { ids });
		} catch (err) {
			this.pages = previous;
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.docs.orderError']();
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
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.docs.toggleError']();
			throw err;
		}
	}
}

function sortDocs(pages: AdminDocPage[], sortBy: DocSortColumn, dir: 'asc' | 'desc'): AdminDocPage[] {
	const sorted = [...pages].sort((a, b) => {
		let cmp = 0;
		switch (sortBy) {
			case 'order':
				cmp = a.sortOrder - b.sortOrder || a.title.localeCompare(b.title) || a.lang.localeCompare(b.lang);
				break;
			case 'title':
				cmp = a.title.localeCompare(b.title) || a.id.localeCompare(b.id);
				break;
			case 'id':
				cmp = a.id.localeCompare(b.id);
				break;
			case 'category':
				cmp = a.category.localeCompare(b.category) || a.title.localeCompare(b.title);
				break;
			case 'lang':
				cmp = a.lang.localeCompare(b.lang) || a.title.localeCompare(b.title);
				break;
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
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
