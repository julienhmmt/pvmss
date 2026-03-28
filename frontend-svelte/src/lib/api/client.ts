import { ApiRequestError, type ApiError } from '$lib/types/api';

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
	const res = await fetch(path, {
		credentials: 'same-origin',
		headers: {
			'Content-Type': 'application/json',
			...options.headers
		},
		...options
	});

	if (res.status === 401) {
		const refreshRes = await fetch('/api/v1/auth/refresh', {
			method: 'POST',
			credentials: 'same-origin'
		});
		if (refreshRes.ok) {
			const retryRes = await fetch(path, {
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json', ...options.headers },
				...options
			});
			if (retryRes.ok) {
				return retryRes.status === 204 ? (undefined as T) : retryRes.json();
			}
			const retryError: ApiError = await retryRes.json().catch(() => ({
				code: 'unknown',
				message: retryRes.statusText
			}));
			throw new ApiRequestError(retryRes.status, retryError);
		}
		if (window.location.pathname !== '/login') {
			window.location.href = '/login';
		}
		throw new ApiRequestError(401, { code: 'unauthorized', message: 'Session expired' });
	}

	if (!res.ok) {
		const error: ApiError = await res.json().catch(() => ({
			code: 'unknown',
			message: res.statusText
		}));
		throw new ApiRequestError(res.status, error);
	}

	return res.status === 204 ? (undefined as T) : res.json();
}

export const api = {
	get: <T>(path: string) => request<T>(path),
	post: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
	put: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
	patch: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
	delete: <T>(path: string) => request<T>(path, { method: 'DELETE' })
};
