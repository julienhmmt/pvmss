import { afterEach, describe, expect, it, vi } from 'vitest';
import { buildConsoleWebSocketURL, consoleTicketErrorMessage, fetchConsoleTicket } from './console';
import { ApiRequestError } from '$lib/shared/api/client';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('fetchConsoleTicket', () => {
	it('POSTs the ticket endpoint and returns the opaque token', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { token: 'opaque-token-abc', expiresInSeconds: 30 }));
		vi.stubGlobal('fetch', fetchMock);

		const token = await fetchConsoleTicket('default', 100);

		expect(token).toBe('opaque-token-abc');
		const [path, init] = fetchMock.mock.calls[0] ?? [];
		expect(path).toBe('/api/v1/vms/default/100/vnc-ticket');
		expect((init as RequestInit).method).toBe('POST');
	});

	it('encodes the cluster and vmid into the path', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { token: 't' })));
		await fetchConsoleTicket('my cluster', 202);
		expect(vi.mocked(fetch).mock.calls[0]?.[0]).toBe('/api/v1/vms/my%20cluster/202/vnc-ticket');
	});

	it('throws ApiRequestError on 403 (non-owner)', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(403, { code: 'forbidden', message: 'not your VM' })));
		await expect(fetchConsoleTicket('default', 100)).rejects.toThrow(ApiRequestError);
	});
});

describe('buildConsoleWebSocketURL', () => {
	it('builds a same-origin wss URL when the page is https', () => {
		vi.stubGlobal('window', { location: { protocol: 'https:', host: 'pvmss.example.com' } });
		const url = buildConsoleWebSocketURL('default', 100, 'opaque-token');
		expect(url).toBe('wss://pvmss.example.com/api/v1/vms/default/100/console/websocket?token=opaque-token');
	});

	it('builds a same-origin ws URL when the page is http', () => {
		vi.stubGlobal('window', { location: { protocol: 'http:', host: 'localhost:50001' } });
		const url = buildConsoleWebSocketURL('default', 100, 'opaque-token');
		expect(url).toBe('ws://localhost:50001/api/v1/vms/default/100/console/websocket?token=opaque-token');
	});

	it('encodes the token and path segments', () => {
		vi.stubGlobal('window', { location: { protocol: 'https:', host: 'h' } });
		const url = buildConsoleWebSocketURL('my cluster', 100, 'token with spaces');
		expect(url).toBe('wss://h/api/v1/vms/my%20cluster/100/console/websocket?token=token%20with%20spaces');
	});
});

describe('consoleTicketErrorMessage', () => {
	it('returns the API message for an ApiRequestError', () => {
		const err = new ApiRequestError(403, 'forbidden', 'not your VM');
		expect(consoleTicketErrorMessage(err, 'fallback')).toBe('not your VM');
	});

	it('returns the fallback for a generic error', () => {
		expect(consoleTicketErrorMessage(new Error('boom'), 'fallback')).toBe('fallback');
	});
});
