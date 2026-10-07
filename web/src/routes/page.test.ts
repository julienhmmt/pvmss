import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount, unmount } from 'svelte';
import TestWrapper from '../test/TestWrapper.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

describe('PVMSS web shell smoke test', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('renders the placeholder heading in the layout', async () => {
		// The layout polls /health, loads the session and reads the version on
		// mount - stub them so the smoke test never touches the network.
		vi.stubGlobal(
			'fetch',
			vi.fn((input: RequestInfo | URL) => {
				const path = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
				if (path.startsWith('/api/v1/auth/me')) {
					return Promise.resolve(jsonResponse(401, { code: 'unauthorized', message: 'not logged in' }));
				}
				if (path.startsWith('/api/v1/public/version')) {
					return Promise.resolve(jsonResponse(200, { version: '0.0.0-test' }));
				}
				if (path.startsWith('/health')) {
					return Promise.resolve(
						jsonResponse(200, { status: 'ok', checks: {}, demoMode: true, timestamp: '2026-01-01T00:00:00Z' })
					);
				}
				return Promise.resolve(jsonResponse(404, { code: 'not_found', message: 'unexpected path' }));
			})
		);

		const app = mount(TestWrapper, { target: document.body });

		expect(document.body.textContent ?? '').toContain('Proxmox VM Self-Service (PVMSS)');
		// Unmount so status.stop() clears the /health interval - otherwise it
		// keeps polling (stubbed) fetch for the rest of the suite.
		await unmount(app);
	});
});
