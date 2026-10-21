import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiRequestError } from './client';
import { reportClientError } from './client-errors';

afterEach(() => vi.unstubAllGlobals());

describe('reportClientError', () => {
	it('logs an API failure with its code, server text and request id, not the localized UI text', () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);

		reportClientError(new ApiRequestError(404, 'not_found', 'vm 999 not found', undefined, { requestId: 'req-9' }));

		const body = JSON.parse((fetchMock.mock.calls[0]?.[1] as { body: string }).body) as { message: string };
		expect(body.message).toBe('not_found: vm 999 not found (requestId req-9)');
	});
});
