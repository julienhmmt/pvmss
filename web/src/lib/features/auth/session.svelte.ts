import { get, post } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import type { Principal } from './login.svelte';

/** SessionStore fetches the current principal from /api/v1/auth/me and exposes
 * the admin flag for the (admin) route group's layout guard. This is a frontend
 * convenience (constitution VI) — the server-side RequireAdmin middleware is
 * the real gate.
 */
export class SessionStore {
	principal = $state.raw<Principal | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.principal = await get<Principal>('/api/v1/auth/me');
		} catch {
			this.principal = null;
		} finally {
			this.loading = false;
		}
	}

	/** Revokes the browser session server-side and clears local state. */
	async logout(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			await post<void>('/api/v1/auth/logout');
		} catch (error: unknown) {
			this.error = error instanceof Error ? error.message : 'logout failed';
		} finally {
			this.principal = null;
			this.loading = false;
		}
	}

	get isAdmin(): boolean {
		return this.principal?.isAdmin ?? false;
	}
}

const SESSION_CONTEXT_KEY = Symbol('session');

export function setSessionContext(): SessionStore {
	const store = new SessionStore();
	setContext(SESSION_CONTEXT_KEY, store);
	return store;
}

export function getSessionContext(): SessionStore {
	return getContext<SessionStore>(SESSION_CONTEXT_KEY);
}
