import { describe, it, expect, vi } from 'vitest';
import { mount, tick } from 'svelte';
import PoolDetailPage from './PoolDetailPage.svelte';
import type { PoolDetail } from './pool-detail.svelte';

const noop = () => undefined;

function buildProps(detail: PoolDetail | null, overrides: Record<string, unknown> = {}) {
	return {
		target: document.body,
		props: {
			detail,
			loading: false,
			error: null,
			notFound: false,
			deleting: false,
			deleteError: null,
			onRefresh: noop,
			onDelete: async () => {},
			onDeleted: noop,
			...overrides
		}
	};
}

function detail(overrides: Partial<PoolDetail> = {}): PoolDetail {
	return {
		name: 'pvmss-alice',
		username: 'pvmss-alice@pve',
		comment: 'Alice pool',
		cluster: 'default',
		managed: true,
		createdAt: '2026-01-01T00:00:00Z',
		quota: { used: 1, allowed: 5 },
		vms: [
			{
				vmid: 100,
				name: 'web-01',
				node: 'pve-01',
				status: 'running',
				uptimeSeconds: 3600,
				cpuCores: 2,
				memoryBytes: 4294967296,
				diskBytes: 34359738368,
				ipAddresses: ['10.0.0.1']
			}
		],
		activity: [
			{
				id: 1,
				actor: 'pvmss-alice@pve',
				cluster: 'default',
				vmid: 100,
				action: 'vm.create',
				timestamp: '2026-01-02T00:00:00Z',
				targetType: '',
				targetId: '',
				detail: '',
				severity: 'info'
			}
		],
		...overrides
	};
}

describe('PoolDetailPage', () => {
	it('renders identity, members and activity for a managed pool', () => {
		mount(PoolDetailPage, buildProps(detail()));
		const text = document.body.textContent ?? '';
		expect(text).toContain('pvmss-alice');
		expect(text).toContain('pvmss-alice@pve');
		expect(text).toContain('web-01');
		expect(text).toContain('10.0.0.1');
		expect(text).toContain('vm.create');
		expect(document.querySelector('[data-testid="pool-detail-delete"]')).not.toBeNull();
		document.body.innerHTML = '';
	});

	it('hides the danger zone for unmanaged pools', () => {
		const unmanaged = detail({ managed: false });
		delete unmanaged.createdAt;
		mount(PoolDetailPage, buildProps(unmanaged));
		const text = document.body.textContent ?? '';
		expect(text).toContain('pvmss-alice');
		expect(text).toContain('web-01');
		expect(document.querySelector('[data-testid="pool-detail-delete"]')).toBeNull();
		document.body.innerHTML = '';
	});

	it('links member VMs to their VM page', () => {
		mount(PoolDetailPage, buildProps(detail()));
		const link = document.querySelector('a[href="/vms/[cluster]/[vmid]"]');
		expect(link).not.toBeNull();
		expect(link?.textContent).toContain('web-01');
		document.body.innerHTML = '';
	});

	it('shows an empty state when the pool has no VMs', () => {
		mount(PoolDetailPage, buildProps(detail({ vms: [], quota: { used: 0, allowed: 5 } })));
		expect(document.body.textContent).toContain('Aucune VM');
		document.body.innerHTML = '';
	});

	it('shows the not-found alert for an unknown pool', () => {
		mount(PoolDetailPage, buildProps(null, { notFound: true }));
		expect(document.body.textContent).toContain("n'existe pas");
		document.body.innerHTML = '';
	});

	it('renders the skeleton while loading', () => {
		mount(PoolDetailPage, buildProps(null, { loading: true }));
		expect(document.querySelector('[data-testid="pool-detail-skeleton"]')).not.toBeNull();
		document.body.innerHTML = '';
	});

	it('opens the delete confirmation and calls onDelete', async () => {
		const onDelete = vi.fn(async () => {});
		const onDeleted = vi.fn();
		mount(PoolDetailPage, buildProps(detail(), { onDelete, onDeleted }));

		const deleteButton = document.querySelector('[data-testid="pool-detail-delete"]') as HTMLButtonElement;
		deleteButton.click();
		await tick();

		const confirmButton = Array.from(document.querySelectorAll('button')).find((button) =>
			button.textContent?.includes('pvmss-alice')
		);
		expect(confirmButton).toBeDefined();
		confirmButton?.click();
		await tick();

		expect(onDelete).toHaveBeenCalled();
		expect(onDeleted).toHaveBeenCalled();
		document.body.innerHTML = '';
	});
});
