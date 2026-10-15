import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { m } from '$lib/paraglide/messages.js';
import LogLevelHarness from './LogLevelHarness.test.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

async function settle(): Promise<void> {
	for (let i = 0; i < 5; i += 1) {
		await Promise.resolve();
		flushSync();
	}
}

describe('LogLevelPanel', () => {
	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
	});

	it('shows the current level, the startup default and the restart note', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { level: 'debug', default: 'info' })));
		const app = mount(LogLevelHarness, { target: document.body });
		await settle();

		const select = document.body.querySelector('select') as HTMLSelectElement;
		expect(select.value).toBe('debug');
		expect(select.options).toHaveLength(4);
		expect(document.body.textContent).toContain(m['admin.logLevel.level.info']());
		expect(document.body.textContent).toContain(m['admin.logLevel.restartNote']());
		await unmount(app);
	});

	it('disables reset at the default level and PUTs the default on reset', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { level: 'debug', default: 'info' }))
			.mockResolvedValueOnce(jsonResponse(200, { level: 'info', default: 'info' }));
		vi.stubGlobal('fetch', fetchMock);
		const app = mount(LogLevelHarness, { target: document.body });
		await settle();

		const reset = document.body.querySelector('button') as HTMLButtonElement;
		expect(reset.disabled).toBe(false);
		reset.click();
		await settle();

		const init = fetchMock.mock.calls[1]?.[1] as RequestInit | undefined;
		expect(JSON.parse(init?.body as string)).toEqual({ level: 'info' });
		expect((document.body.querySelector('button') as HTMLButtonElement).disabled).toBe(true);
		await unmount(app);
	});
});
