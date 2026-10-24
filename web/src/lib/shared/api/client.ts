import { resolveErrorMessage } from '#lib/features/chrome/errorMessage.js';

interface ErrorEnvelope {
	code: string;
	message: string;
	retryAfterSeconds?: number;
	privilege?: string;
	path?: string;
}

const CSRF_COOKIE_NAME = 'pvmss_csrf';
/** Dispatched on `globalThis` when the server no longer knows the session
 *  (401 `unauthenticated`), so the shell can end it once; the client itself
 *  keeps no session state. A wrong password is 401 `invalid_credentials` and
 *  must not sign the user out. */
export const UNAUTHORIZED_EVENT = 'pvmss-unauthorized';
const CSRF_HEADER_NAME = 'X-CSRF-Token';

export class ApiRequestError extends Error {
	readonly status: number;
	readonly code: string;
	/** Present on 429 responses that carry a guard countdown (contracts/cluster-refresh.md). */
	readonly retryAfterSeconds?: number | undefined;

	readonly privilege?: string | undefined;
	readonly path?: string | undefined;
	/** The server's own (English) message, kept for support; never the UI text. */
	readonly serverMessage: string;
	/** X-Request-Id of the failed response, to find it in the server log. */
	readonly requestId?: string | undefined;

	constructor(
		status: number,
		code: string,
		message: string,
		retryAfterSeconds?: number,
		details: Pick<ErrorEnvelope, 'privilege' | 'path'> & { requestId?: string | undefined } = {}
	) {
		// `message` is what every store shows, so it is localized here, once.
		super(resolveErrorMessage(code, message, details));
		this.name = 'ApiRequestError';
		this.serverMessage = message;
		this.requestId = details.requestId;
		this.status = status;
		this.code = code;
		this.retryAfterSeconds = retryAfterSeconds;
		this.privilege = details.privilege;
		this.path = details.path;
	}
}

/** The single network entry point (constitution XIII) - every API call goes through this. */
export async function get<T>(path: string, init: RequestInit = {}): Promise<T> {
	return request<T>(path, init);
}

/** Sends a JSON request through the shared API error model. */
export async function post<T>(path: string, body?: unknown): Promise<T> {
	if (body === undefined) return request<T>(path, withCSRF({ method: 'POST' }));
	return request<T>(path, withCSRF({ method: 'POST', body: JSON.stringify(body) }));
}

/** Sends a DELETE request through the shared API error model. */
export async function del<T>(path: string): Promise<T> {
	return request<T>(path, withCSRF({ method: 'DELETE' }));
}

/** Sends a PUT request with a JSON body through the shared API error model. */
export async function put<T>(path: string, body: unknown): Promise<T> {
	return request<T>(path, withCSRF({ method: 'PUT', body: JSON.stringify(body) }));
}

/** Sends a PATCH request with a JSON body through the shared API error model. */
export async function patch<T>(path: string, body: unknown): Promise<T> {
	return request<T>(path, withCSRF({ method: 'PATCH', body: JSON.stringify(body) }));
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
	const response = await fetch(path, {
		...options,
		headers: {
			Accept: 'application/json',
			...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }),
			...options.headers
		}
	});
	if (!response.ok) {
		let envelope: ErrorEnvelope = { code: 'unknown_error', message: 'request failed' };
		try {
			envelope = (await response.json()) as ErrorEnvelope;
		} catch {
			// Body wasn't JSON (or was empty) - keep the generic envelope.
		}

		if (response.status === 403 && envelope.code === 'invalid_csrf_token' && typeof globalThis !== 'undefined') {
			globalThis.location.reload();
		}
		if (response.status === 401 && envelope.code === 'unauthenticated' && typeof globalThis.dispatchEvent === 'function') {
			globalThis.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
		}

		const requestId = response.headers.get('X-Request-Id') ?? undefined;
		throw new ApiRequestError(response.status, envelope.code, envelope.message, envelope.retryAfterSeconds, { ...envelope, requestId });
	}
	if (response.status === 204) return undefined as T;
	return (await response.json()) as T;
}

/** Returns the value of a cookie by name, or an empty string. */
function getCookie(name: string): string {
	if (typeof document === 'undefined') {
		return '';
	}

	const match = new RegExp(String.raw`(?:^|;\s*)${name}=([^;]*)`).exec(document.cookie);
	return match?.[1] ? decodeURIComponent(match[1]) : '';
}

/**
 * Adds the X-CSRF-Token header to mutating requests. Exported for the few
 * callers that must use fetch directly (multipart uploads, blob downloads).
 */
export function withCSRF(options: RequestInit): RequestInit {
	const token = getCookie(CSRF_COOKIE_NAME);
	if (!token) {
		return options;
	}

	return {
		...options,
		headers: {
			...options.headers,
			[CSRF_HEADER_NAME]: token
		}
	};
}
