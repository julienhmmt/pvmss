import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiRequestError, del, get, patch, post, put } from './client';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('get', () => {
	it('returns parsed JSON on 200', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { ok: true })));
		await expect(get('/api/v1/foo')).resolves.toEqual({ ok: true });
	});

	it('returns undefined on 204', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
		await expect(get('/api/v1/foo')).resolves.toBeUndefined();
	});

	it('throws ApiRequestError on non-2xx', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(404, { code: 'not_found', message: 'gone' })));
		await expect(get('/api/v1/foo')).rejects.toThrow(ApiRequestError);
	});

	it('throws with generic envelope when body is not JSON', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('not json', { status: 500 })));
		await expect(get('/api/v1/foo')).rejects.toThrow('request failed');
	});
});

describe('post', () => {
	it('sends POST without body when body is undefined', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
		vi.stubGlobal('fetch', fetchMock);
		await post('/api/v1/foo');
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		expect(init.method).toBe('POST');
		expect(init.body).toBeUndefined();
	});

	it('sends POST with JSON body', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
		vi.stubGlobal('fetch', fetchMock);
		await post('/api/v1/foo', { name: 'test' });
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		expect(init.method).toBe('POST');
		expect(init.body).toBe(JSON.stringify({ name: 'test' }));
	});
});

describe('del', () => {
	it('sends DELETE', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);
		await del('/api/v1/foo');
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		expect(init.method).toBe('DELETE');
	});
});

describe('put', () => {
	it('sends PUT with JSON body', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
		vi.stubGlobal('fetch', fetchMock);
		await put('/api/v1/foo', { name: 'test' });
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		expect(init.method).toBe('PUT');
		expect(init.body).toBe(JSON.stringify({ name: 'test' }));
	});
});

describe('patch', () => {
	it('sends PATCH with JSON body', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
		vi.stubGlobal('fetch', fetchMock);
		await patch('/api/v1/foo', { name: 'test' });
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		expect(init.method).toBe('PATCH');
		expect(init.body).toBe(JSON.stringify({ name: 'test' }));
	});
});

describe('CSRF', () => {
	it('adds X-CSRF-Token header when cookie is present', async () => {
		vi.stubGlobal('document', { cookie: 'pvmss_csrf=test-token; path=/' });
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
		vi.stubGlobal('fetch', fetchMock);
		await post('/api/v1/foo', { x: 1 });
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		const headers = init.headers as Record<string, string>;
		expect(headers['X-CSRF-Token']).toBe('test-token');
	});

	it('omits X-CSRF-Token header when cookie is absent', async () => {
		vi.stubGlobal('document', { cookie: '' });
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
		vi.stubGlobal('fetch', fetchMock);
		await post('/api/v1/foo', { x: 1 });
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
		const headers = init.headers as Record<string, string>;
		expect(headers['X-CSRF-Token']).toBeUndefined();
	});
});

describe('ApiRequestError', () => {
	it('carries status, code, message, and retryAfterSeconds', () => {
		const err = new ApiRequestError(429, 'rate_limited', 'slow down', 30);
		expect(err.status).toBe(429);
		expect(err.code).toBe('rate_limited');
		expect(err.message).toBe('slow down');
		expect(err.retryAfterSeconds).toBe(30);
		expect(err.name).toBe('ApiRequestError');
	});
});
