import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import ClusterDetailPage from './ClusterDetailPage.svelte';
import type { AdminCluster } from './clusters.svelte';

function jsonResponse(body: unknown): Response {
	return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });
}

const cluster: AdminCluster = {
	name: 'east',
	displayName: 'East Campus',
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
	snippetStorage: '',
	cloudInitWriteEnabled: false
};

const node = (clusterKey: string, name: string) => ({
	clusterKey, cluster: clusterKey, name, status: 'online', vmCount: 3, vmRunningCount: 2,
	cpuCores: 8, cpuUsage: 0.25, memoryTotalBytes: 1024, memoryUsedBytes: 512
});
const storage = (clusterLabel: string, name: string) => ({
	cluster: clusterLabel, name, type: 'rbd', shared: true, usedBytes: 1, totalBytes: 2, percent: 50
});

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

	it('shows connection, last test, and only this cluster inventory', async () => {
		vi.stubGlobal('fetch', vi.fn((url: string) => Promise.resolve(jsonResponse(
			url.endsWith('/dashboard')
				? { nodes: [node('east', 'pve-east'), node('west', 'pve-west')], storages: [storage('East Campus', 'ceph-east'), storage('west', 'ceph-west')] }
				: [cluster]
		))));
		mount(ClusterDetailPage, { target: document.body, props: { name: 'east' } });
		await flush();

		const text = document.body.textContent ?? '';
		expect(text).toContain('https://east.invalid:8006/api2/json');
		expect(text).toContain('Verification skipped');
		expect(text).toContain('dial tcp: timeout');
		expect(text).toContain('9.2.21');
		expect(text).toContain('pve-east');
		expect(text).toContain('ceph-east');
		expect(text).not.toContain('pve-west');
		expect(text).not.toContain('ceph-west');
	});

	it('reports an unknown cluster', async () => {
		vi.stubGlobal('fetch', vi.fn((url: string) => Promise.resolve(jsonResponse(url.endsWith('/dashboard') ? { nodes: [], storages: [] } : [cluster]))));
		mount(ClusterDetailPage, { target: document.body, props: { name: 'nope' } });
		await flush();
		expect(document.body.textContent).toContain('does not exist');
	});
});
