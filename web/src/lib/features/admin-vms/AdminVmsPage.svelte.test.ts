import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { mount, tick } from 'svelte';
import AdminVmsPage from './AdminVmsPage.svelte';
import {
	VmListStore,
	getVmListContext,
	type VmListItem,
	type VmListResult
} from '#lib/features/vms/list.svelte.js';
import { setLocale } from '#lib/paraglide/runtime.js';

vi.mock('$app/paths', () => ({
	resolve: (path: string) => path
}));

vi.mock('#lib/features/vms/list.svelte.js', async (importOriginal) => {
	const mod = await importOriginal<typeof import('#lib/features/vms/list.svelte.js')>();
	return { ...mod, getVmListContext: vi.fn() };
});

// Rows derive their display status from the tray; an empty one is enough here.
vi.mock('#lib/features/tasks/tasks.svelte.js', () => ({
	getTaskTrayContext: vi.fn(() => ({ tasks: [] }))
}));

function machine(overrides: Partial<VmListItem>): VmListItem {
	return {
		cluster: 'default',
		clusterDisplayName: 'Default',
		vmid: 100,
		name: 'web-01',
		node: 'pve-01',
		status: 'running',
		pool: 'pvmss-alice',
		tags: ['pvmss'],
		ostype: 'l26',
		cpuCores: 2,
		memoryTotal: 4294967296,
		...overrides
	};
}

function makeResult(items: VmListItem[]): VmListResult {
	return {
		items,
		total: items.length,
		page: 1,
		pageSize: 10,
		availableNodes: ['pve-01'],
		availablePools: ['pvmss-alice', 'pvmss-bob']
	};
}

function makeStore(result: VmListResult): VmListStore {
	const store = new VmListStore({ scope: 'all', initialQuery: '?scope=all', navigate: () => {} });
	store.result = result;
	return store;
}

function text(): string {
	return document.body.textContent ?? '';
}

describe('AdminVmsPage', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
	});

	it('lists every pool and names the total', () => {
		const store = makeStore(
			makeResult([
				machine({ vmid: 100, name: 'web-01', pool: 'pvmss-alice' }),
				machine({ vmid: 103, name: 'cache-01', pool: 'pvmss-bob', status: 'stopped' })
			])
		);
		vi.mocked(getVmListContext).mockReturnValue(store);

		mount(AdminVmsPage, { target: document.body });

		expect(text()).toContain('web-01');
		expect(text()).toContain('cache-01');
		expect(text()).toContain('pvmss-alice');
		expect(text()).toContain('pvmss-bob');
		expect(document.querySelector('[data-testid="admin-vm-count"]')?.textContent).toBe('2 machines');
	});

	it('opens the admin detail route from the row link', () => {
		vi.mocked(getVmListContext).mockReturnValue(makeStore(makeResult([machine({})])));

		mount(AdminVmsPage, { target: document.body });

		const link = document.querySelector('[data-testid="admin-vm-row-link"]');
		expect(link?.getAttribute('href')).toBe('/admin/vms/[cluster]/[vmid]');
	});

	it('labels a VM with no pool', () => {
		vi.mocked(getVmListContext).mockReturnValue(makeStore(makeResult([machine({ pool: '' })])));

		mount(AdminVmsPage, { target: document.body });

		expect(text()).toContain('No pool');
	});

	it('asks before force-stopping a machine', async () => {
		const store = makeStore(makeResult([machine({ status: 'running' })]));
		const rowAction = vi.spyOn(store, 'rowAction').mockResolvedValue({ ok: true });
		vi.mocked(getVmListContext).mockReturnValue(store);

		mount(AdminVmsPage, { target: document.body });
		(document.querySelector('[data-testid="admin-vm-stop"]') as HTMLButtonElement).click();
		await tick();

		expect(text()).toContain('Force stop web-01?');
		expect(rowAction).not.toHaveBeenCalled();

		const confirm = document.querySelector('#confirm-dialog-title');
		expect(confirm?.textContent).toContain('Force stop web-01?');
	});
});
