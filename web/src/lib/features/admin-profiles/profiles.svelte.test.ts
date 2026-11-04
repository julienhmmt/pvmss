import { describe, expect, it, vi } from 'vitest';
import { AdminProfilesStore, vcpuTotal } from './profiles.svelte';
import { m } from '#lib/paraglide/messages.js';

const testProfiles = [
	{ id: 'small', label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi', enabled: true, description: '' },
	{ id: 'large', label: 'Large', sockets: 1, cpuCores: 4, memoryMB: 8192, diskGB: 80, bus: 'virtio', enabled: false, description: '' },
	{ id: 'medium', label: 'Medium', sockets: 1, cpuCores: 2, memoryMB: 4096, diskGB: 40, bus: 'scsi', enabled: true, description: '' }
];

describe('AdminProfilesStore filteredProfiles', () => {
	it('returns all profiles when no filters are set', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		expect(store.filteredProfiles.length).toBe(3);
	});

	it('filters by search on label', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.search = 'large';
		expect(store.filteredProfiles.length).toBe(1);
		expect(store.filteredProfiles[0]?.label).toBe('Large');
	});

	it('filters by search on id', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.search = 'small';
		expect(store.filteredProfiles.length).toBe(1);
		expect(store.filteredProfiles[0]?.id).toBe('small');
	});

	it('filters by bus', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.busFilter = 'virtio';
		expect(store.filteredProfiles.length).toBe(1);
		expect(store.filteredProfiles[0]?.bus).toBe('virtio');
	});

	it('filters by enabled state', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.enabledFilter = 'disabled';
		expect(store.filteredProfiles.length).toBe(1);
		expect(store.filteredProfiles[0]?.enabled).toBe(false);
	});

	it('sorts by label ascending then descending', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.sortBy = 'label';
		store.sortDir = 'asc';
		expect(store.filteredProfiles.map((p) => p.label)).toEqual(['Large', 'Medium', 'Small']);
		store.sortDir = 'desc';
		expect(store.filteredProfiles.map((p) => p.label)).toEqual(['Small', 'Medium', 'Large']);
	});

	it('sorts by cpuCores', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.sortBy = 'cpuCores';
		store.sortDir = 'asc';
		expect(store.filteredProfiles.map((p) => p.cpuCores)).toEqual([1, 2, 4]);
	});

	it('resetFilters clears all filters', () => {
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		store.search = 'small';
		store.busFilter = 'scsi';
		store.enabledFilter = 'enabled';
		store.resetFilters();
		expect(store.filteredProfiles.length).toBe(3);
	});
});

describe('vcpuTotal', () => {
	it('multiplies sockets by cores', () => {
		expect(vcpuTotal({ sockets: 2, cpuCores: 4 })).toBe(8);
	});

	it('equals cores when sockets is 1', () => {
		expect(vcpuTotal({ sockets: 1, cpuCores: 4 })).toBe(4);
	});
});

describe('AdminProfilesStore create', () => {
	function jsonResponse(status: number, body: unknown): Response {
		return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
	}

	it('sends the sockets the admin entered', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(201, { id: 'p', label: 'p', sockets: 2, cpuCores: 4, memoryMB: 2048, diskGB: 20, bus: 'scsi', enabled: true })
		);
		vi.stubGlobal('fetch', fetchMock);

		await expect(new AdminProfilesStore().create('p', 4, 2048, 20, 'scsi', 2)).resolves.toBe(true);

		const init = fetchMock.mock.calls[0]![1] as RequestInit;
		const body = JSON.parse(init.body as string) as { sockets: number };
		expect(body.sockets).toBe(2);
		vi.unstubAllGlobals();
	});

	it('returns false and sets saveError on failure instead of throwing', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(400, { code: 'bad', message: 'duplicate label' })));
		const store = new AdminProfilesStore();
		await expect(store.create('p', 4, 2048, 20, 'scsi', 2)).resolves.toBe(false);
		expect(store.saveError).toBe(m['error.generic']());
		expect(store.profiles).toHaveLength(0);
		vi.unstubAllGlobals();
	});

	it('update() returns false and sets saveError on failure', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { code: 'oops', message: 'server broke' })));
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;
		await expect(store.update('small', 'p', 4, 2048, 20, 'scsi', 2)).resolves.toBe(false);
		expect(store.saveError).toBe(m['error.generic']());
		expect(store.profiles[0]?.label).toBe('Small');
		vi.unstubAllGlobals();
	});
});

describe('AdminProfilesStore description', () => {
	function jsonResponse(status: number, body: unknown): Response {
		return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
	}

	function sentBody(fetchMock: ReturnType<typeof vi.fn>): { description?: string } {
		const init = fetchMock.mock.calls[0]![1] as RequestInit;
		return JSON.parse(init.body as string) as { description?: string };
	}

	it('create() sends the description and stores the returned row', async () => {
		const row = { id: 'p', label: 'p', sockets: 1, cpuCores: 4, memoryMB: 2048, diskGB: 20, bus: 'scsi', enabled: true, description: 'Web apps' };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, row));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminProfilesStore();

		await expect(store.create('p', 4, 2048, 20, 'scsi', 1, 'Web apps')).resolves.toBe(true);

		expect(sentBody(fetchMock).description).toBe('Web apps');
		expect(store.profiles[0]?.description).toBe('Web apps');
		vi.unstubAllGlobals();
	});

	it('update() sends the description and replaces the row with the returned one', async () => {
		const row = { ...testProfiles[0]!, description: 'Light web' };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, row));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;

		await expect(store.update('small', 'Small', 1, 2048, 20, 'scsi', 1, 'Light web')).resolves.toBe(true);

		expect(sentBody(fetchMock).description).toBe('Light web');
		expect(store.profiles.find((p) => p.id === 'small')?.description).toBe('Light web');
		vi.unstubAllGlobals();
	});

	it('update() sends an empty description to clear it', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ...testProfiles[0]!, description: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminProfilesStore();
		store.profiles = testProfiles;

		await expect(store.update('small', 'Small', 1, 2048, 20, 'scsi', 1, '')).resolves.toBe(true);

		expect(sentBody(fetchMock).description).toBe('');
		vi.unstubAllGlobals();
	});
});
