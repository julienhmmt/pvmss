import { afterEach, describe, expect, it, vi } from 'vitest';
import { clusterTabParam, parseClusterTab, testClusterConnection } from './cluster-detail.js';

afterEach(() => vi.unstubAllGlobals());

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('parseClusterTab', () => {
	it.each([
		['status', 'status'],
		['infrastructure', 'infrastructure'],
		['connection', 'connection'],
		['bogus', 'status'],
		['', 'status'],
		[null, 'status']
	] as const)('maps %s to %s', (raw, want) => {
		expect(parseClusterTab(raw)).toBe(want);
	});
});

describe('clusterTabParam', () => {
	it.each([
		['status', null],
		['infrastructure', 'infrastructure'],
		['connection', 'connection']
	] as const)('serializes %s to %s', (tab, want) => {
		expect(clusterTabParam(tab)).toBe(want);
	});
});

describe('testClusterConnection', () => {
	it('POSTs the test endpoint with the encoded cluster name', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { status: 'ok' }));
		vi.stubGlobal('fetch', fetchMock);

		const result = await testClusterConnection('my cluster');

		expect(result.status).toBe('ok');
		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/admin/clusters/my%20cluster/test');
		expect(init?.method).toBe('POST');
	});
});
