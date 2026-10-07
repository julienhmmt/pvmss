import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { mount, tick } from 'svelte';
import MachineList from './MachineList.svelte';
import { VmListStore, getVmListContext, type VmListItem, type VmListResult, type VmQuota } from './list.svelte';
import { VmBulkSelection, getVmBulkContext } from './bulk.svelte';
import { setLocale } from '$lib/paraglide/runtime.js';

vi.mock('./list.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./list.svelte')>();
	return { ...mod, getVmListContext: vi.fn() };
});
vi.mock('./bulk.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./bulk.svelte')>();
	return { ...mod, getVmBulkContext: vi.fn() };
});
// Rows derive their display status from the tray; an empty one is enough
// here - the ledger and power-action getters already degrade gracefully
// outside the app shell.
vi.mock('$lib/features/tasks/tasks.svelte', () => ({
	getTaskTrayContext: vi.fn(() => ({ tasks: [] }))
}));

const vmItem: VmListItem = {
	cluster: 'default',
	clusterDisplayName: 'Default',
	vmid: 100,
	name: 'web-01',
	node: 'pve-01',
	status: 'running',
	pool: 'pool-alice',
	tags: ['pvmss'],
	ostype: 'l26',
	cpuCores: 2,
	memoryTotal: 4294967296
};

function makeResult(total: number, quota?: VmQuota): VmListResult {
	const result: VmListResult = {
		items: [vmItem],
		total,
		page: 1,
		pageSize: 10,
		availableNodes: ['pve-01']
	};
	if (quota !== undefined) result.quota = quota;
	return result;
}

function makeStore(initialQuery: string, result: VmListResult): VmListStore {
	const store = new VmListStore({ scope: 'mine', initialQuery, navigate: () => {} });
	store.result = result;
	return store;
}

function vmCountText(): string | null {
	return document.querySelector('[data-testid="vm-count"]')?.textContent ?? null;
}

describe('MachineList machine count', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders "N of M machines" when the list is filtered and a quota is present', () => {
		vi.mocked(getVmListContext).mockReturnValue(makeStore('?search=web', makeResult(2, { used: 7, allowed: -1 })));
		mount(MachineList, { target: document.body });
		expect(vmCountText()).toBe('2 of 7 machines');
	});

	it('falls back to the plain count when the filtered list has no quota', () => {
		vi.mocked(getVmListContext).mockReturnValue(makeStore('?status=running', makeResult(2)));
		mount(MachineList, { target: document.body });
		expect(vmCountText()).toBe('2 machines');
	});

	it('renders the plain count when the list is unfiltered, even with a quota', () => {
		vi.mocked(getVmListContext).mockReturnValue(makeStore('', makeResult(7, { used: 7, allowed: -1 })));
		mount(MachineList, { target: document.body });
		expect(vmCountText()).toBe('7 machines');
	});
});

describe('MachineList sorting and selection', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('sorts by CPU from the resources column header', async () => {
		const store = makeStore('', makeResult(1));
		const setSort = vi.spyOn(store, 'setSort').mockImplementation(() => {});
		vi.mocked(getVmListContext).mockReturnValue(store);
		mount(MachineList, { target: document.body });

		const cpuSort = document.querySelector('[data-testid="vm-sort-cpu"]');
		expect(cpuSort).not.toBeNull();
		cpuSort?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
		await tick();

		expect(setSort).toHaveBeenCalledWith('cpu');
	});

	it('offers select-all-matching when the result spans pages, and selects them all', async () => {
		const bulk = new VmBulkSelection();
		vi.mocked(getVmBulkContext).mockReturnValue(bulk);
		const store = makeStore('', makeResult(25));
		const fetchAll = vi.spyOn(store, 'fetchAllMatching').mockResolvedValue([vmItem]);
		vi.mocked(getVmListContext).mockReturnValue(store);
		mount(MachineList, { target: document.body });

		const toggle = document.querySelector('[data-testid="vm-select-mode"]');
		expect(toggle).not.toBeNull();
		toggle?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
		await tick();

		const matching = document.querySelector('[data-testid="vm-bulk-select-all-matching"]');
		expect(matching?.textContent).toContain('25');
		matching?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
		await tick();
		await Promise.resolve();

		expect(fetchAll).toHaveBeenCalled();
		expect(bulk.selectedCount).toBe(1);
	});
});
