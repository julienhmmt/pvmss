interface ErrorEnvelope {
	code: string;
	message: string;
}

export class ApiRequestError extends Error {
	readonly status: number;
	readonly code: string;

	constructor(status: number, code: string, message: string) {
		super(message);
		this.name = 'ApiRequestError';
		this.status = status;
		this.code = code;
	}
}

/** The single network entry point (constitution XIII) — every API call goes through this. */
export async function get<T>(path: string): Promise<T> {
	const response = await fetch(path);

	if (!response.ok) {
		let envelope: ErrorEnvelope = { code: 'unknown_error', message: 'request failed' };
		try {
			envelope = (await response.json()) as ErrorEnvelope;
		} catch {
			// Body wasn't JSON (or was empty) — keep the generic envelope.
		}
		throw new ApiRequestError(response.status, envelope.code, envelope.message);
	}

	return (await response.json()) as T;
}
