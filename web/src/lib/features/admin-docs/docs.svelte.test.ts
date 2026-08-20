import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminDocsStore } from './docs.svelte';
import { ApiRequestError } from '$lib/shared/api/client';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

const samplePage = {
	id: 'getting-started',
	lang: 'en',
	title: 'Getting Started',
	category: 'Guides',
	bodyMd: '# Getting Started\n\nWelcome.',
	audience: 'user' as const,
	enabled: true,
	isSystem: false,
	sortOrder: 0
};

describe('AdminDocsStore', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('loads all documentation pages from the admin endpoint', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, [samplePage]));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminDocsStore();

		await store.load();
		expect(store.pages).toEqual([samplePage]);
		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/admin/docs',
			expect.objectContaining({ headers: { Accept: 'application/json' } })
		);
	});

	it('records an error message when load fails', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValueOnce(jsonResponse(500, { code: 'internal_error', message: 'boom' }))
		);
		const store = new AdminDocsStore();

		await store.load();
		expect(store.pages).toEqual([]);
		expect(store.error).toBe('boom');
	});

	it('creates a page and appends it to the list', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(201, samplePage));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminDocsStore();

		const created = await store.create({
			title: 'Getting Started',
			lang: 'en',
			category: 'Guides',
			bodyMd: '# Getting Started\n\nWelcome.',
			audience: 'user'
		});
		expect(created).toEqual(samplePage);
		expect(store.pages).toHaveLength(1);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/admin/docs',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({
					title: 'Getting Started',
					lang: 'en',
					category: 'Guides',
					bodyMd: '# Getting Started\n\nWelcome.',
					audience: 'user'
				})
			})
		);
	});

	it('updates a page by id and lang, replacing the matching entry', async () => {
		const updated = { ...samplePage, title: 'Getting Started v2' };
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, updated));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminDocsStore();
		store.pages = [samplePage];

		await store.update('getting-started', 'en', {
			title: 'Getting Started v2',
			lang: 'en',
			category: 'Guides',
			bodyMd: '# Getting Started\n\nWelcome.',
			audience: 'user',
			enabled: true,
			sortOrder: 0
		});
		expect(store.pages[0]?.title).toBe('Getting Started v2');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/admin/docs/getting-started/en',
			expect.objectContaining({ method: 'PUT' })
		);
	});

	it('deletes a page by id and lang, removing it from the list', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { status: 'deleted' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminDocsStore();
		store.pages = [samplePage];

		await store.remove('getting-started', 'en');
		expect(store.pages).toHaveLength(0);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/admin/docs/getting-started/en',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('toggles a page enabled flag by id and lang', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(
				jsonResponse(200, { id: 'getting-started', lang: 'en', enabled: false })
			);
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminDocsStore();
		store.pages = [samplePage];

		await store.toggle('getting-started', 'en', false);
		expect(store.pages[0]?.enabled).toBe(false);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/admin/docs/getting-started/en/toggle',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ enabled: false })
			})
		);
	});

	it('surfaces a saveError and rethrows when create fails with a conflict', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValueOnce(
				jsonResponse(409, { code: 'duplicate_page', message: 'a documentation page with this title already exists for this language' })
			)
		);
		const store = new AdminDocsStore();

		await expect(
			store.create({ title: 'dup', lang: 'en', category: 'X', bodyMd: 'body', audience: 'user' })
		).rejects.toBeInstanceOf(ApiRequestError);
		expect(store.saveError).toBe('a documentation page with this title already exists for this language');
		expect(store.saving).toBe(false);
	});

	describe('filteredPages', () => {
		const testPages = [
			{ id: 'getting-started', lang: 'en', title: 'Getting Started', category: 'Guides', bodyMd: '', audience: 'user' as const, enabled: true, isSystem: false, sortOrder: 0 },
			{ id: 'admin-guide', lang: 'en', title: 'Admin Guide', category: 'Admin', bodyMd: '', audience: 'admin' as const, enabled: true, isSystem: false, sortOrder: 1 },
			{ id: 'guide-fr', lang: 'fr', title: 'Guide de démarrage', category: 'Guides', bodyMd: '', audience: 'user' as const, enabled: false, isSystem: false, sortOrder: 2 }
		];

		it('returns all pages when no filters are set', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			expect(store.filteredPages.length).toBe(3);
		});

		it('filters by search on title', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.search = 'admin';
			expect(store.filteredPages.length).toBe(1);
			expect(store.filteredPages[0]?.title).toBe('Admin Guide');
		});

		it('filters by search on slug/id', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.search = 'getting';
			expect(store.filteredPages.length).toBe(1);
			expect(store.filteredPages[0]?.id).toBe('getting-started');
		});

		it('filters by category', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.categoryFilter = 'Guides';
			expect(store.filteredPages.length).toBe(2);
		});

		it('filters by language', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.langFilter = 'fr';
			expect(store.filteredPages.length).toBe(1);
			expect(store.filteredPages[0]?.lang).toBe('fr');
		});

		it('filters by audience', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.audienceFilter = 'admin';
			expect(store.filteredPages.length).toBe(1);
			expect(store.filteredPages[0]?.audience).toBe('admin');
		});

		it('sorts by title ascending then descending', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.sortBy = 'title';
			store.sortDir = 'asc';
			expect(store.filteredPages.map((p) => p.title)).toEqual(['Admin Guide', 'Getting Started', 'Guide de démarrage']);
			store.sortDir = 'desc';
			expect(store.filteredPages.map((p) => p.title)).toEqual(['Guide de démarrage', 'Getting Started', 'Admin Guide']);
		});

		it('resetFilters clears all filters', () => {
			const store = new AdminDocsStore();
			store.pages = testPages;
			store.search = 'admin';
			store.categoryFilter = 'Admin';
			store.langFilter = 'en';
			store.resetFilters();
			expect(store.filteredPages.length).toBe(3);
		});
	});
});
