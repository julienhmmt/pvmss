import { describe, it, expect, beforeAll, afterEach, vi, type MockInstance } from 'vitest';
import { mount, tick } from 'svelte';
import VmBulkActionBar from './VmBulkActionBar.svelte';
import { VmBulkSelection, getVmBulkContext } from './bulk.svelte';
import { VmListStore, getVmListContext, type VmListItem, type VmListResult } from './list.svelte';
import { setLocale } from '$lib/paraglide/runtime.js';

vi.mock('./list.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./list.svelte')>();
	return { ...mod, getVmListContext: vi.fn() };
});
vi.mock('./bulk.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./bulk.svelte')>();
	return { ...mod, getVmBulkContext: vi.fn() };
});

const vmItem: VmListItem = {
	cluster: 'default',
	clusterDisplayName: 'Demo Cluster Alpha',
	vmid: 102,
	name: 'db-01',
	node: 'pve-01',
	status: 'running',
	pool: 'pool-alice',
	tags: ['pvmss'],
	ostype: 'l26',
	cpuCores: 2,
	memoryTotal: 4294967296
};

/** Mount the bar with one machine selected and a spy on the submit call. */
function setup(): { bulk: VmBulkSelection; submit: MockInstance } {
	const bulk = new VmBulkSelection();
	bulk.toggle({ cluster: vmItem.cluster, vmid: vmItem.vmid });
	const submit: MockInstance = vi.spyOn(bulk, 'submitBulkAction').mockResolvedValue({ results: [] });
	vi.mocked(getVmBulkContext).mockReturnValue(bulk);

	const store = new VmListStore({ scope: 'mine', initialQuery: '', navigate: () => {} });
	const result: VmListResult = {
		items: [vmItem],
		total: 1,
		page: 1,
		pageSize: 10,
		availableNodes: ['pve-01']
	};
	store.result = result;
	vi.mocked(getVmListContext).mockReturnValue(store);

	mount(VmBulkActionBar, { target: document.body });
	return { bulk, submit };
}

async function apply(): Promise<void> {
	const form = document.querySelector('[data-testid="vm-bulk-action-bar"] form');
	if (form === null) throw new Error('bulk form not rendered');
	form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
	await tick();
	await Promise.resolve();
}

// The destructive path is gated by bulkConfirmationKind, which is unit-tested
// in bulk.svelte.test.ts. It cannot be driven from here: the action select is
// a shared Select using bind:value, and a dispatched change event does not
// update a Svelte 5 select binding under jsdom. This file covers what is
// drivable - the default (non-destructive) action still submits immediately.
describe('VmBulkActionBar', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.clearAllMocks();
	});

	it('submits a non-destructive action without a confirmation', async () => {
		const { submit } = setup();
		await apply();

		expect(document.querySelector('[data-testid="vm-bulk-confirm"]')).toBeNull();
		expect(submit).toHaveBeenCalledWith('start');
	});
});
