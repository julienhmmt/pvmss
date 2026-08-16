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
		vi.useRealTimers();
	});

	it('loads nodes with vmCount and refreshedAt on success', async () => {
		const nodes = [
			{
				name: 'pve-node-01',
				status: 'online',
				cpuCores: 32,
				cpuUsage: 0.42,
				memoryTotal: 100,
				memoryUsed: 50,
				storageTotal: 100,
				storageUsed: 50,
				vmCount: 9
			}
		];
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, { nodes, refreshedAt: '2026-08-01T12:00:00Z' }))
		);

		const store = new NodesStore();
		await store.load();

		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
		expect(store.nodes).toEqual(nodes);
		expect(store.nodes[0]!.vmCount).toBe(9);
		expect(store.refreshedAt).toBe('2026-08-01T12:00:00Z');
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
		expect(store.errorCode).toBe('cluster_unreachable');
		expect(store.errorStatus).toBe(502);
		expect(store.nodes).toEqual([]);
	});

	it('sets an unauthenticated code on a 401 response', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(401, { code: 'unauthenticated', message: 'authentication required' }))
		);

		const store = new NodesStore();
		await store.load();

		expect(store.loading).toBe(false);
		expect(store.error).toBe('authentication required');
		expect(store.errorCode).toBe('unauthenticated');
		expect(store.errorStatus).toBe(401);
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

	it('refresh updates refreshedAt on success', async () => {
		const fetchMock = vi.fn();
		// First call: POST /cluster/refresh -> 200
		// Second call: GET /cluster/nodes -> 200
		fetchMock
			.mockResolvedValueOnce(jsonResponse(200, { refreshedAt: '2026-08-01T12:00:05Z' }))
			.mockResolvedValueOnce(
				jsonResponse(200, {
					nodes: [
						{
							name: 'pve-node-01',
							status: 'online',
							cpuCores: 32,
							cpuUsage: 0.42,
							memoryTotal: 100,
							memoryUsed: 50,
							storageTotal: 100,
							storageUsed: 50,
							vmCount: 10
						}
					],
					refreshedAt: '2026-08-01T12:00:05Z'
				})
			);
		vi.stubGlobal('fetch', fetchMock);

		const store = new NodesStore();
		await store.refresh();

		expect(store.refreshing).toBe(false);
		expect(store.refreshError).toBeNull();
		expect(store.refreshedAt).toBe('2026-08-01T12:00:05Z');
		expect(store.nodes[0]!.vmCount).toBe(10);
	});

	it('refresh sets refreshDisabled on refresh_too_soon (429)', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(429, {
					code: 'refresh_too_soon',
					message: 'please wait before refreshing again',
					retryAfterSeconds: 3
				})
			)
		);

		const store = new NodesStore();
		await store.refresh();

		expect(store.refreshing).toBe(false);
		expect(store.refreshDisabled).toBe(true);
		expect(store.refreshError).toBe('please wait before refreshing again');
	});

	it('refresh auto re-enables the button once retryAfterSeconds elapses (quickstart step 6)', async () => {
		vi.useFakeTimers();
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(429, {
					code: 'refresh_too_soon',
					message: 'please wait before refreshing again',
					retryAfterSeconds: 3
				})
			)
		);

		const store = new NodesStore();
		await store.refresh();
		expect(store.refreshDisabled).toBe(true);

		await vi.advanceTimersByTimeAsync(2999);
		expect(store.refreshDisabled).toBe(true);

		await vi.advanceTimersByTimeAsync(1);
		expect(store.refreshDisabled).toBe(false);
		expect(store.refreshError).toBeNull();
	});

	it('refresh falls back to a 5s re-enable delay when retryAfterSeconds is absent', async () => {
		vi.useFakeTimers();
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(429, { code: 'refresh_too_soon', message: 'please wait before refreshing again' })
			)
		);

		const store = new NodesStore();
		await store.refresh();
		expect(store.refreshDisabled).toBe(true);

		await vi.advanceTimersByTimeAsync(5000);
		expect(store.refreshDisabled).toBe(false);
	});

	it('refresh sets a generic error on other ApiRequestError failures', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(502, { code: 'cluster_unreachable', message: 'refresh failed: cluster is not reachable' })
			)
		);

		const store = new NodesStore();
		await store.refresh();

		expect(store.refreshing).toBe(false);
		expect(store.refreshDisabled).toBe(false);
		expect(store.refreshError).toBe('refresh failed: cluster is not reachable');
	});

	it('clearRefreshDisabled resets the disabled and error state', () => {
		const store = new NodesStore();
		store.refreshDisabled = true;
		store.refreshError = 'please wait';

		store.clearRefreshDisabled();

		expect(store.refreshDisabled).toBe(false);
		expect(store.refreshError).toBeNull();
	});
});
