import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LogLevelStore } from './logLevel.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('LogLevelStore', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('loads the current level and the startup default', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { level: 'info', default: 'info' })));
		const store = new LogLevelStore();
		await store.load();
		expect(store.level).toBe('info');
		expect(store.defaultLevel).toBe('info');
		expect(store.isDefault).toBe(true);
	});

	it('sets a level with PUT and reflects the server response', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { level: 'debug', default: 'info' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new LogLevelStore();
		await store.set('debug');
		expect(store.level).toBe('debug');
		expect(store.isDefault).toBe(false);
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(init?.method).toBe('PUT');
		expect(JSON.parse(init?.body as string)).toEqual({ level: 'debug' });
		expect(store.saved).toBe(true);
	});

	it('resets to the startup default', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { level: 'debug', default: 'warn' }))
			.mockResolvedValueOnce(jsonResponse(200, { level: 'warn', default: 'warn' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new LogLevelStore();
		await store.load();
		await store.reset();
		const init = fetchMock.mock.calls[1]?.[1] as RequestInit | undefined;
		expect(JSON.parse(init?.body as string)).toEqual({ level: 'warn' });
		expect(store.isDefault).toBe(true);
	});

	it('keeps the previous level and exposes an error when the PUT fails', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { level: 'info', default: 'info' }))
			.mockResolvedValueOnce(jsonResponse(400, { code: 'invalid_request', message: 'level must be one of debug, info, warn, error' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new LogLevelStore();
		await store.load();
		await store.set('debug');
		expect(store.level).toBe('info');
		expect(store.saveError).not.toBeNull();
		expect(store.saved).toBe(false);
	});
});
