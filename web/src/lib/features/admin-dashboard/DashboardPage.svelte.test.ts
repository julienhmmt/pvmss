import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import { goto } from '$app/navigation';
import { m } from '#lib/paraglide/messages.js';
import DashboardPage from './DashboardPage.svelte';
import { getDashboardContext, type DashboardSummary } from './dashboard.svelte';
import type { DashboardAlert } from './dashboard-alerts';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/paths', () => ({
	resolve: (path: string, params?: Record<string, string>) =>
		Object.entries(params ?? {}).reduce((resolved, [key, value]) => resolved.replace(`[${key}]`, encodeURIComponent(value)), path)
}));
vi.mock('./dashboard.svelte', () => ({ getDashboardContext: vi.fn() }));

const store = {
	summary: {
		alerts: [],
		nodes: [
			{
				clusterKey: 'default',
				cluster: 'Default cluster',
				name: 'miniquarium',
				status: 'online',
				vmCount: 2,
				vmRunningCount: 1,
				pvmssVMCount: 1,
				cpuCores: 8,
				cpuUsage: 0.25,
				memoryTotalBytes: 8_000_000_000,
				memoryUsedBytes: 2_000_000_000
			}
		],
		nodeCount: 1,
		vmCount: 10,
		vmStatusCounts: { running: 4, paused: 1, stopped: 5, other: 0 },
		pvmssVMCount: 3,
		pvmssVMStatusCounts: { running: 1, paused: 0, stopped: 2, other: 0 },
		otherVMCount: 7,
		otherNodeCount: 0,
		catalogUnavailable: false,
		storages: [],
		recentChanges: [],
		version: '0.4.0-test',
		refreshedAt: '2026-01-01T00:00:00Z'
	},
	loading: false,
	error: null,
	errorCode: null,
	lastLoadedAt: 0,
	load: async (): Promise<void> => {},
	refreshIfStale: async (): Promise<void> => {},
	completeOnboarding: async (): Promise<void> => {}
};

beforeEach(() => {
	document.body.innerHTML = '';
	vi.mocked(getDashboardContext).mockReturnValue(store);
});

describe('DashboardPage VM summary', () => {
	it('shows PVMSS VM statuses separately from other VMs', () => {
		mount(DashboardPage, { target: document.body });

		const summary = document.querySelector('[data-testid="dashboard-vm-status"]');
		const otherVMs = document.querySelector('[data-testid="dashboard-vm-other-count"]');

		expect(summary?.textContent).toMatch(/PVMSS/i);
		expect(document.querySelector('[data-testid="dashboard-vm-total"]')?.textContent).toContain('3');
		expect(document.querySelector('[data-testid="dashboard-vm-status-running"]')?.textContent).toContain('1');
		expect(document.querySelector('[data-testid="dashboard-vm-status-stopped"]')?.textContent).toContain('2');
		expect(otherVMs?.textContent).toContain('7');
		expect(otherVMs?.textContent).toMatch(/pvmss/i);
	});

	it('opens the selected node in its cluster-scoped admin detail page', () => {
		mount(DashboardPage, { target: document.body });

		document.querySelector<HTMLButtonElement>('[data-testid="dashboard-node-card"]')?.click();

		expect(goto).toHaveBeenCalledWith('/admin/nodes/default/miniquarium?from=dashboard');
	});

	it('leads with the create-user quick action and no placeholder slots', () => {
		mount(DashboardPage, { target: document.body });

		expect(document.querySelector('[data-testid="dashboard-create-user"]')?.getAttribute('href')).toBe('/admin/pools?create=1');
		expect(document.querySelectorAll('[data-testid="dashboard-quick-action-slot"]')).toHaveLength(0);
	});

	it('sums pending quota requests across clusters on the quick action', () => {
		const alerts: DashboardAlert[] = [
			{ kind: 'quota_requests_pending', severity: 'warning', cluster: 'Default', clusterKey: 'default', used: 2 },
			{ kind: 'quota_requests_pending', severity: 'warning', cluster: 'Edge', clusterKey: 'edge', used: 1 },
			{ kind: 'node_offline', severity: 'critical', cluster: 'Default', clusterKey: 'default', subject: 'pve-01' }
		];
		vi.mocked(getDashboardContext).mockReturnValue({ ...store, summary: { ...store.summary, alerts } });
		mount(DashboardPage, { target: document.body });

		const tile = document.querySelector('[data-testid="dashboard-quota-requests"]');
		expect(tile?.getAttribute('href')).toBe('/admin/quota-requests');
		expect(tile?.textContent).toContain('3');
	});

	it('names recent changes in plain language', () => {
		const recentChanges = [{ id: 1, actor: 'alice', cluster: 'default', vmid: 101, action: 'console_open', timestamp: '2026-01-01T00:00:00Z', targetType: 'vm', targetId: '101' }];
		vi.mocked(getDashboardContext).mockReturnValue({ ...store, summary: { ...store.summary, recentChanges } });
		mount(DashboardPage, { target: document.body });

		const list = document.querySelector('[data-testid="dashboard-recent-changes"]')?.textContent ?? '';
		expect(list).toContain(m['activity.action.console_open']());
		expect(list).not.toContain('console_open');
	});

	it('lists only the five fullest storages', () => {
		const storages = [10, 80, 30, 95, 50, 20, 60].map((percent) => ({
			cluster: 'Default cluster', node: 'miniquarium', name: `s${percent}`, type: 'dir', shared: false, percent, usedBytes: percent, totalBytes: 100
		}));
		vi.mocked(getDashboardContext).mockReturnValue({ ...store, summary: { ...store.summary, storages } });
		mount(DashboardPage, { target: document.body });

		const names = [...document.querySelectorAll('[data-testid="dashboard-storage"]')].map((el) => el.textContent?.trim().split(/\s+/)[0]);
		expect(names).toEqual(['s95', 's80', 's60', 's50', 's30']);
	});
});

describe('DashboardPage node states', () => {
	const base = store.summary.nodes[0]!;

	function mountWith(summary: Partial<DashboardSummary>): void {
		vi.mocked(getDashboardContext).mockReturnValue({ ...store, summary: { ...store.summary, ...summary } });
		mount(DashboardPage, { target: document.body });
	}

	it('badges each state and lists what an incomplete node misses', () => {
		mountWith({
			nodes: [
				{ ...base, name: 'down', state: 'offline' },
				{ ...base, name: 'half', state: 'incomplete', missing: ['bridge', 'iso_or_template'] },
				{ ...base, name: 'idle', state: 'ready', vmCount: 0 },
				{ ...base, name: 'old', state: 'disabled_with_vms', vmCount: 10, pvmssVMCount: 1 }
			]
		});

		const states = [...document.querySelectorAll('[data-testid="dashboard-node-state"]')].map((el) => el.textContent?.trim());
		expect(states).toEqual([m['admin.dashboard.nodeState.offline'](), m['admin.dashboard.nodeState.incomplete'](), m['admin.dashboard.nodeNoVMs'](), m['admin.dashboard.nodeState.disabled_with_vms']({ count: 1 })]);
		const half = document.querySelector('[data-node="half"]')?.textContent ?? '';
		expect(half).toContain(m['admin.dashboard.missing.bridge']());
		expect(half).toContain(m['admin.dashboard.missing.iso_or_template']());
	});

	it('shows cluster headers only with several clusters', () => {
		mountWith({ nodes: [base] });
		expect(document.querySelectorAll('[data-testid="dashboard-node-group"]')).toHaveLength(0);

		document.body.innerHTML = '';
		mountWith({ nodes: [base, { ...base, clusterKey: 'edge', cluster: 'Edge', name: 'pve-edge' }] });
		expect(document.querySelectorAll('[data-testid="dashboard-node-group"]')).toHaveLength(2);
	});

	it('links the other nodes line to the nodes page and shows the catalog banner', () => {
		mountWith({ otherNodeCount: 3, catalogUnavailable: true });

		const other = document.querySelector('[data-testid="dashboard-other-nodes"]');
		expect(other?.getAttribute('href')).toBe('/admin/nodes');
		expect(other?.textContent).toContain('3');
		expect(document.querySelector('[data-testid="dashboard-catalog-unavailable"]')).not.toBeNull();
	});
});
