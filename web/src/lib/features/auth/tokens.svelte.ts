import { getContext, setContext } from 'svelte';
import { get, post, del, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';

export type TokenScope = 'read' | 'read_write';

export interface ApiToken {
	id: string;
	label: string;
	scope: TokenScope;
	createdAt: string;
	lastUsedAt?: string;
}

interface TokenListResponse {
	tokens: ApiToken[];
}

interface CreateTokenResponse extends ApiToken {
	value: string;
}

export class TokensStore {
	tokens = $state.raw<ApiToken[]>([]);
	loading = $state.raw(false);
	creating = $state.raw(false);
	revoking = $state.raw<Record<string, boolean>>({});
	error = $state.raw<string | null>(null);
	lastCreatedValue = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const response = await get<TokenListResponse>('/api/v1/auth/tokens');
			this.tokens = response.tokens;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['profile.tokens.errorLoad']();
		} finally {
			this.loading = false;
		}
	}

	async create(label: string, scope: TokenScope): Promise<boolean> {
		this.error = null;
		this.creating = true;
		try {
			const created = await post<CreateTokenResponse>('/api/v1/auth/tokens', { label, scope });
			this.lastCreatedValue = created.value;
			await this.load();
			return true;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['profile.tokens.errorCreate']();
			return false;
		} finally {
			this.creating = false;
		}
	}

	async revoke(id: string): Promise<boolean> {
		this.error = null;
		this.revoking[id] = true;
		try {
			await del(`/api/v1/auth/tokens/${id}`);
			await this.load();
			return true;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['profile.tokens.errorRevoke']();
			return false;
		} finally {
			this.revoking[id] = false;
		}
	}

	dismissCreatedValue(): void {
		this.lastCreatedValue = null;
	}
}

const TOKENS_CONTEXT_KEY = Symbol('tokens');

/** Called once, by the route that owns this state (constitution VII: no module singletons). */
export function setTokensContext(): TokensStore {
	const store = new TokensStore();
	setContext(TOKENS_CONTEXT_KEY, store);
	return store;
}

export function getTokensContext(): TokensStore {
	return getContext<TokensStore>(TOKENS_CONTEXT_KEY);
}
