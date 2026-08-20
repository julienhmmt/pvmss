import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminCatalogStore, type AdminBridge } from './admin-catalog.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const bridges: AdminBridge[] = [
	{ name: 'vmbr0', node: 'node-a', active: true, comment: '', enabled: false },
	{ name: 'vmbr0', node: 'node-b', active: true, comment: '', enabled: false }
];

describe('AdminCatalogStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('toggles the exact bridge node and name pair', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(200, { node: 'node-a', name: 'vmbr0', enabled: true })
		);
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminCatalogStore();
		store.bridges = bridges;

		await store.toggleBridge('node-a', 'vmbr0', true);

		expect(store.bridges).toEqual([
			{ ...bridges[0], enabled: true },
			bridges[1]
		]);
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({
			cluster: 'default',
			node: 'node-a',
			name: 'vmbr0',
			enabled: true
		});
	});
});
