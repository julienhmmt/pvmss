import { afterEach, describe, expect, it, vi } from 'vitest';
import { cloneVm, suggestCopyName } from './clone';

afterEach(() => vi.unstubAllGlobals());

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('suggestCopyName', () => {
	it('suggests -copy when no copy exists', () => {
		expect(suggestCopyName('web-01')).toBe('web-01-copy');
	});

	it('moves to -copy-2 when -copy is taken', () => {
		expect(suggestCopyName('web-01', ['web-01-copy'])).toBe('web-01-copy-2');
	});

	it('skips every taken numbered copy', () => {
		expect(suggestCopyName('web-01', ['web-01-copy', 'web-01-copy-2', 'other'])).toBe('web-01-copy-3');
	});

	it('ignores unrelated taken names', () => {
		expect(suggestCopyName('web-01', ['db-01', 'web-01'])).toBe('web-01-copy');
	});
});

describe('cloneVm', () => {
	it('POSTs the clone endpoint with the encoded cluster and the new name', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(202, { cluster: 'default', vmid: 100, name: 'web-01-copy', node: 'pve-01', upid: 'UPID:x' }));
		vi.stubGlobal('fetch', fetchMock);

		const result = await cloneVm('my cluster', 100, 'web-01-copy');

		expect(result.upid).toBe('UPID:x');
		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/vms/my%20cluster/100/clone');
		expect(init?.method).toBe('POST');
		expect(JSON.parse(init?.body as string)).toEqual({ name: 'web-01-copy' });
	});
});
