import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import { goto } from '$app/navigation';
import { m } from '#lib/paraglide/messages.js';
import DashboardPage from './DashboardPage.svelte';
import { getDashboardContext } from './dashboard.svelte';

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
		storages: [],
		recentChanges: [],
		version: '0.4.0-test',
		refreshedAt: '2026-01-01T00:00:00Z'
	},
	loading: false,
	error: null,
	errorCode: null,
	load: async (): Promise<void> => {}
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

	it('leads with the create-user quick action and four reserved slots', () => {
		mount(DashboardPage, { target: document.body });

		expect(document.querySelector('[data-testid="dashboard-create-user"]')?.getAttribute('href')).toBe('/admin/pools?create=1');
		expect(document.querySelectorAll('[data-testid="dashboard-quick-action-slot"]')).toHaveLength(4);
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
