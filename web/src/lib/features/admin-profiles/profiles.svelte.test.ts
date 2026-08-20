import { describe, expect, it } from 'vitest';
import { AdminProfilesStore } from './profiles.svelte';

const testProfiles = [
	{ id: 'small', label: 'Small', cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi', enabled: true },
	{ id: 'large', label: 'Large', cpuCores: 4, memoryMB: 8192, diskGB: 80, bus: 'virtio', enabled: false },
	{ id: 'medium', label: 'Medium', cpuCores: 2, memoryMB: 4096, diskGB: 40, bus: 'scsi', enabled: true }
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
