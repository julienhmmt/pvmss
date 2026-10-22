import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiRequestError } from './client';
import { installClientErrorReporting, reportClientError } from './client-errors';

afterEach(() => vi.unstubAllGlobals());

describe('reportClientError', () => {
	it('logs an API failure with its code, server text and request id, not the localized UI text', () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);

		reportClientError(new ApiRequestError(404, 'not_found', 'vm 999 not found', undefined, { requestId: 'req-9' }));

		const body = JSON.parse((fetchMock.mock.calls[0]?.[1] as { body: string }).body) as { message: string };
		expect(body.message).toBe('not_found: vm 999 not found (requestId req-9)');
	});

	it('formats API errors without requestId, standard Errors, and non-Error values while swallowing network failures', () => {
		const fetchMock = vi.fn().mockRejectedValue(new Error('offline'));
		vi.stubGlobal('fetch', fetchMock);

		reportClientError(new ApiRequestError(500, 'internal_error', 'boom'));
		reportClientError(new Error('unexpected failure'));
		reportClientError('raw-failure');

		const messages: string[] = fetchMock.mock.calls.map(
			(call) => (JSON.parse((call[1] as { body: string }).body) as { message: string }).message
		);
		expect(messages).toEqual(['internal_error: boom', 'unexpected failure', 'raw-failure']);
	});

	it('installs global error and unhandledrejection listeners idempotently', () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);

		installClientErrorReporting();
		installClientErrorReporting();

		globalThis.dispatchEvent(new ErrorEvent('error', { message: 'script error' }));
		const rejection = new Event('unhandledrejection') as PromiseRejectionEvent;
		Object.defineProperty(rejection, 'reason', { value: new Error('rejected promise') });
		globalThis.dispatchEvent(rejection);

		const messages: string[] = fetchMock.mock.calls.map(
			(call) => (JSON.parse((call[1] as { body: string }).body) as { message: string }).message
		);
		expect(messages).toEqual(['script error', 'rejected promise']);
	});
});
