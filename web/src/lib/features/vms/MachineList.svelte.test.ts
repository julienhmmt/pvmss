import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { mount } from 'svelte';
import MachineList from './MachineList.svelte';
import { VmListStore, getVmListContext, type VmListItem, type VmListResult, type VmQuota } from './list.svelte';
import { setLocale } from '$lib/paraglide/runtime.js';

vi.mock('./list.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./list.svelte')>();
	return { ...mod, getVmListContext: vi.fn() };
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
