import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import ClusterDetailPage from './ClusterDetailPage.svelte';
import type { ClusterDetail } from './cluster-detail.js';

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const detail: ClusterDetail = {
	name: 'east',
	displayName: 'East Campus',
	oidcRealm: '',
	url: 'https://east.invalid:8006/api2/json',
	tlsInsecureSkipVerify: true,
	tokenId: 'pvmss@pve!service',
	tokenSet: true,
	removedAt: null,
	lastTestStatus: 'unreachable',
	lastTestAt: '2026-10-20T10:00:00Z',
	lastTestMessage: 'dial tcp: timeout',
	proxmoxVersion: '9.2.21',
	nodeCount: 1,
	vmCount: 3,
	inventory: {
		stale: false,
		refreshedAt: '2026-11-13T09:00:00Z',
		nodes: [
			{
				clusterKey: 'east',
				cluster: 'East Campus',
				name: 'pve-east',
				status: 'online',
				vmCount: 3,
				vmRunningCount: 2,
				cpuCores: 8,
				cpuUsage: 0.25,
				memoryTotalBytes: 1024,
				memoryUsedBytes: 512
			},
			{
				clusterKey: 'east',
				cluster: 'East Campus',
				name: 'pve-empty',
				status: 'offline',
				vmCount: 0,
				vmRunningCount: 0,
				cpuCores: 4,
				cpuUsage: 0,
				memoryTotalBytes: 0,
				memoryUsedBytes: 0
			}
		],
		storages: [
			{ cluster: 'East Campus', name: 'ceph-east', type: 'rbd', shared: true, usedBytes: 1, totalBytes: 2, percent: 50 }
		],
		alerts: [
			{ kind: 'node_offline', severity: 'critical', cluster: 'East Campus', clusterKey: 'east', subject: 'pve-empty' }
		],
		vmCount: 3,
		vmStatusCounts: { running: 2, paused: 0, stopped: 1, other: 0 }
	}
};

function stubDetail(body: unknown = detail, status = 200): void {
	vi.stubGlobal(
		'fetch',
		vi.fn(() => Promise.resolve(jsonResponse(body, status)))
	);
}

async function flush(): Promise<void> {
	for (let i = 0; i < 5; i++) await tick();
	await new Promise((r) => setTimeout(r, 0));
	await tick();
}

describe('ClusterDetailPage', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.unstubAllGlobals();
	});

	it('shows the status tab with alerts, last test and inventory counts', async () => {
		stubDetail();
		mount(ClusterDetailPage, { target: document.body, props: { name: 'east' } });
		await flush();

		const text = document.body.textContent ?? '';
		expect(text).toContain('dial tcp: timeout');
		expect(text).toContain('pve-empty');
		expect(document.querySelector('[data-testid="dashboard-alert"]')).not.toBeNull();
	});

	it('lists every node on the infrastructure tab, including zero-VM ones', async () => {
		stubDetail();
		mount(ClusterDetailPage, { target: document.body, props: { name: 'east' } });
		await flush();

		(document.querySelector('#tab-infrastructure') as HTMLButtonElement).click();
		await flush();

		const text = document.body.textContent ?? '';
		expect(text).toContain('pve-east');
		expect(text).toContain('pve-empty');
		expect(text).toContain('ceph-east');
	});

	it('shows the connection grid on the connection tab', async () => {
		stubDetail();
		mount(ClusterDetailPage, { target: document.body, props: { name: 'east' } });
		await flush();

		(document.querySelector('#tab-connection') as HTMLButtonElement).click();
		await flush();

		const text = document.body.textContent ?? '';
		expect(text).toContain('https://east.invalid:8006/api2/json');
		expect(text).toContain('Verification skipped');
		expect(text).toContain('9.2.21');
	});

	it('reports an unknown cluster', async () => {
		stubDetail({ code: 'not_found', message: 'cluster not found' }, 404);
		mount(ClusterDetailPage, { target: document.body, props: { name: 'nope' } });
		await flush();
		expect(document.body.textContent).toContain('does not exist');
	});
});
