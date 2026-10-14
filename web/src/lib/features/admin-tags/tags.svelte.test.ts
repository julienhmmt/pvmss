import { describe, expect, it, vi } from 'vitest';
import { AdminTagsStore } from './tags.svelte';

const testTags = [
	{ name: 'pvmss', color: '#4f46e5', vmCount: 10, protected: true },
	{ name: 'team-web', color: '#22c55e', vmCount: 5, protected: false },
	{ name: 'team-api', color: '#f59e0b', vmCount: 3, protected: false }
];

describe('AdminTagsStore filteredTags', () => {
	it('returns all tags when no filters are set', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		expect(store.filteredTags.length).toBe(3);
	});

	it('filters by search on name', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		store.search = 'web';
		expect(store.filteredTags.length).toBe(1);
		expect(store.filteredTags[0]?.name).toBe('team-web');
	});

	it('filters by protected state', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		store.protectedFilter = 'protected';
		expect(store.filteredTags.length).toBe(1);
		expect(store.filteredTags[0]?.protected).toBe(true);
	});

	it('filters by unprotected state', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		store.protectedFilter = 'unprotected';
		expect(store.filteredTags.length).toBe(2);
		expect(store.filteredTags.every((t) => !t.protected)).toBe(true);
	});

	it('sorts by name ascending then descending', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		store.sortBy = 'name';
		store.sortDir = 'asc';
		expect(store.filteredTags.map((t) => t.name)).toEqual(['pvmss', 'team-api', 'team-web']);
		store.sortDir = 'desc';
		expect(store.filteredTags.map((t) => t.name)).toEqual(['team-web', 'team-api', 'pvmss']);
	});

	it('sorts by vmCount descending', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		store.sortBy = 'vmCount';
		store.sortDir = 'desc';
		expect(store.filteredTags.map((t) => t.vmCount)).toEqual([10, 5, 3]);
	});

	it('resetFilters clears all filters', () => {
		const store = new AdminTagsStore();
		store.tags = testTags;
		store.search = 'team';
		store.protectedFilter = 'protected';
		store.resetFilters();
		expect(store.filteredTags.length).toBe(3);
	});
});

describe('AdminTagsStore create contract', () => {
	function jsonResponse(status: number, body: unknown): Response {
		return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
	}

	it('returns true and appends the tag on success', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(201, { name: 'new', color: '#fff', vmCount: 0, protected: false })));
		const store = new AdminTagsStore();
		await expect(store.create('new', '#fff')).resolves.toBe(true);
		expect(store.tags.at(-1)?.name).toBe('new');
		vi.unstubAllGlobals();
	});

	it('returns false and sets saveError on failure instead of throwing', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(400, { code: 'bad', message: 'name already used' })));
		const store = new AdminTagsStore();
		await expect(store.create('pvmss', '#fff')).resolves.toBe(false);
		expect(store.saveError).toBe('name already used');
		vi.unstubAllGlobals();
	});
});
