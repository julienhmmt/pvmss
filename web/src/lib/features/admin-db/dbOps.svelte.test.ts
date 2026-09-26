import { afterEach, describe, expect, it, vi } from 'vitest';
import { DbOpsStore } from './dbOps.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('DbOpsStore CSRF', () => {
	it('sends the CSRF token on the import upload and on the confirm', async () => {
		vi.stubGlobal('document', { cookie: 'pvmss_csrf=tok; path=/' });
		const preview = { stagingToken: 's1', expiresAt: '2026-01-01T00:00:00Z', tables: [], ignoredTables: [] };
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, preview))
			.mockResolvedValueOnce(jsonResponse(200, { status: 'imported', tables: [] }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new DbOpsStore();
		await store.uploadImport(new File(['x'], 'export.db'));
		expect(store.importError).toBeNull();
		await store.confirmImport();
		expect(store.confirmError).toBeNull();

		for (const call of fetchMock.mock.calls) {
			const headers = (call[1] as RequestInit).headers as Record<string, string>;
			expect(headers['X-CSRF-Token']).toBe('tok');
		}
		const uploadHeaders = (fetchMock.mock.calls[0]?.[1] as RequestInit).headers as Record<string, string>;
		// Multipart: the browser must set the boundary, so no explicit Content-Type.
		expect(uploadHeaders['Content-Type']).toBeUndefined();
	});
});
