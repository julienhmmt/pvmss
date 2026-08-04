import { describe, it, expect, vi, afterEach } from 'vitest';
import { NodesStore } from './nodes.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

describe('NodesStore', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('loads nodes on success', async () => {
		const nodes = [
			{
				name: 'pve-node-01',
				status: 'online',
				cpuCores: 32,
				cpuUsage: 0.42,
				memoryTotal: 100,
				memoryUsed: 50,
				storageTotal: 100,
				storageUsed: 50
			}
		];
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, { nodes }))
		);

		const store = new NodesStore();
		await store.load();

		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
		expect(store.nodes).toEqual(nodes);
	});

	it('sets error on an ApiRequestError-shaped failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(502, { code: 'cluster_unreachable', message: 'cluster is not reachable' })
			)
		);

		const store = new NodesStore();
		await store.load();

		expect(store.loading).toBe(false);
		expect(store.error).toBe('cluster is not reachable');
		expect(store.nodes).toEqual([]);
	});

	it('sets a generic error on a network failure', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network down')));

		const store = new NodesStore();
		await store.load();

		expect(store.loading).toBe(false);
		expect(store.error).toBe('failed to load cluster nodes');
		expect(store.nodes).toEqual([]);
	});
});
