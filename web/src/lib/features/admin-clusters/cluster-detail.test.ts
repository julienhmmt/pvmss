import { afterEach, describe, expect, it, vi } from 'vitest';
import { clusterTabParam, fetchClusterTests, parseClusterTab, parseTestHistoryLimit, testClusterConnection } from './cluster-detail.js';

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

describe('parseTestHistoryLimit', () => {
	it.each([
		['25', 25],
		['100', 100],
		['7', 5],
		['abc', 5],
		[null, 5]
	] as const)('maps %s to %s', (raw, want) => {
		expect(parseTestHistoryLimit(raw)).toBe(want);
	});
});

describe('fetchClusterTests', () => {
	it('GETs the history endpoint with the limit and unwraps tests', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { tests: [{ testedAt: 't', status: 'ok', durationMs: 12 }] }));
		vi.stubGlobal('fetch', fetchMock);

		const tests = await fetchClusterTests('my cluster', 25);

		expect(tests).toHaveLength(1);
		const [path] = (fetchMock.mock.calls[0] ?? []) as [string];
		expect(path).toBe('/api/v1/admin/clusters/my%20cluster/tests?limit=25');
	});
});
