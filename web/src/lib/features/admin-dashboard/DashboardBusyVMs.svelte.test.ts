import { beforeEach, describe, expect, it } from 'vitest';
import { flushSync, mount } from 'svelte';
import DashboardBusyVMs from './DashboardBusyVMs.svelte';
import type { BusyVM } from './dashboard.svelte';

const vm: BusyVM = {
	clusterKey: 'east',
	cluster: 'East',
	node: 'n1',
	vmid: 101,
	name: 'web-01',
	pool: 'alice',
	cpuPercent: 87,
	memoryPercent: 60,
	memoryUsedBytes: 2 ** 31,
	memoryTotalBytes: 2 ** 32
};

beforeEach(() => {
	document.body.innerHTML = '';
});

describe('DashboardBusyVMs', () => {
	it('lists the busiest VMs with their value and a link', () => {
		mount(DashboardBusyVMs, { target: document.body, props: { topCPU: [vm], topMemory: [vm] } });
		flushSync();

		const cpu = document.querySelector('[data-testid="dashboard-busy-cpu"]');
		expect(cpu?.textContent).toContain('87%');
		expect(cpu?.querySelector('a')?.getAttribute('href')).toContain('/admin/vms/east/101');
		expect(document.querySelector('[data-testid="dashboard-busy-memory"]')?.textContent).toContain('60%');
	});

	it('says so when no PVMSS VM runs', () => {
		mount(DashboardBusyVMs, { target: document.body, props: { topCPU: [], topMemory: [] } });
		flushSync();

		expect(document.querySelector('[data-testid="dashboard-busy-none"]')).not.toBeNull();
	});
});
