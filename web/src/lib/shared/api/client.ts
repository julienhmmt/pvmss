interface ErrorEnvelope {
	code: string;
	message: string;
	retryAfterSeconds?: number;
}

export class ApiRequestError extends Error {
	readonly status: number;
	readonly code: string;
	/** Present on 429 responses that carry a guard countdown (contracts/cluster-refresh.md). */
	readonly retryAfterSeconds?: number | undefined;

	constructor(status: number, code: string, message: string, retryAfterSeconds?: number) {
		super(message);
		this.name = 'ApiRequestError';
		this.status = status;
		this.code = code;
		this.retryAfterSeconds = retryAfterSeconds;
	}
}

/** The single network entry point (constitution XIII) — every API call goes through this. */
export async function get<T>(path: string): Promise<T> {
	return request<T>(path);
}

/** Sends a JSON request through the shared API error model. */
export async function post<T>(path: string, body?: unknown): Promise<T> {
	if (body === undefined) return request<T>(path, { method: 'POST' });
	return request<T>(path, { method: 'POST', body: JSON.stringify(body) });
}

/** Sends a DELETE request through the shared API error model. */
export async function del<T>(path: string): Promise<T> {
	return request<T>(path, { method: 'DELETE' });
}

/** Sends a PUT request with a JSON body through the shared API error model. */
export async function put<T>(path: string, body: unknown): Promise<T> {
	return request<T>(path, { method: 'PUT', body: JSON.stringify(body) });
}

/** Sends a PATCH request with a JSON body through the shared API error model. */
export async function patch<T>(path: string, body: unknown): Promise<T> {
	return request<T>(path, { method: 'PATCH', body: JSON.stringify(body) });
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
	const response = await fetch(path, {
		...options,
		headers: { Accept: 'application/json', ...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }) }
	});
	if (!response.ok) {
		let envelope: ErrorEnvelope = { code: 'unknown_error', message: 'request failed' };
		try {
			envelope = (await response.json()) as ErrorEnvelope;
		} catch {
			// Body wasn't JSON (or was empty) — keep the generic envelope.
		}
		throw new ApiRequestError(response.status, envelope.code, envelope.message, envelope.retryAfterSeconds);
	}
	if (response.status === 204) return undefined as T;
	return (await response.json()) as T;
}
