import { get, post, put, del, ApiRequestError } from '#lib/shared/api/client.js';
import { setContext, getContext } from 'svelte';
import { m } from '#lib/paraglide/messages.js';

/** A DB-backed PVMSS administrator, as returned by /api/v1/admin/accounts. */
export interface LocalAccount {
	id: number;
	username: string;
	disabled: boolean;
	failedAttempts: number;
	lockedUntil?: string;
	createdAt: string;
	updatedAt: string;
	createdBy: string;
	lastLoginAt?: string;
}

interface AccountsResponse {
	accounts: LocalAccount[];
	max: number;
	count: number;
}

/** Local account error codes the API returns (server/internal/httpapi/admin_accounts.go). */
const ACCOUNT_ERROR_CODES: Partial<Record<string, () => string>> = {
	owner_only: m['admin.accounts.errorOwnerOnly'],
	account_limit_reached: m['admin.accounts.errorLimitReached'],
	username_taken: m['admin.accounts.errorUsernameTaken'],
	invalid_username: m['admin.accounts.errorInvalidUsername'],
	weak_password: m['admin.accounts.errorWeakPassword'],
	not_found: m['admin.accounts.errorNotFound']
};

function translateAccountError(error: unknown, fallback: () => string): string {
	if (error instanceof ApiRequestError) {
		const translated = ACCOUNT_ERROR_CODES[error.code];
		if (translated) return translated();
		return error.message;
	}
	return fallback();
}

/**
 * AdminAccountsStore owns the owner-only local administrator list and its
 * mutations. Responses are $state.raw: API data, not form edits. Every
 * mutation reloads the list silently so server-side facts (lock countdown,
 * last login, cap usage) stay authoritative.
 */
export class AdminAccountsStore {
	accounts = $state.raw<LocalAccount[]>([]);
	max = $state.raw(0);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);

	get count(): number {
		return this.accounts.length;
	}

	get atLimit(): boolean {
		return this.max > 0 && this.accounts.length >= this.max;
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.apply(await get<AccountsResponse>('/api/v1/admin/accounts'));
		} catch (err) {
			this.error = translateAccountError(err, m['admin.accounts.loadError']);
		} finally {
			this.loading = false;
		}
	}

	async create(username: string, password: string): Promise<boolean> {
		return this.mutate(async () => {
			await post<LocalAccount>('/api/v1/admin/accounts', { username, password });
		}, m['admin.accounts.createError']);
	}

	async resetPassword(id: number, password: string): Promise<boolean> {
		return this.mutate(async () => {
			await put<void>(`/api/v1/admin/accounts/${id}/password`, { password });
		}, m['admin.accounts.resetError']);
	}

	async setDisabled(id: number, disabled: boolean): Promise<boolean> {
		return this.mutate(async () => {
			await post<void>(`/api/v1/admin/accounts/${id}/toggle`, { disabled });
		}, m['admin.accounts.toggleError']);
	}

	async remove(id: number): Promise<boolean> {
		return this.mutate(async () => {
			await del<void>(`/api/v1/admin/accounts/${id}`);
		}, m['admin.accounts.deleteError']);
	}

	clearSaveError(): void {
		this.saveError = null;
	}

	private async mutate(action: () => Promise<void>, fallback: () => string): Promise<boolean> {
		this.saving = true;
		this.saveError = null;
		try {
			await action();
			await this.reload();
			return true;
		} catch (err) {
			this.saveError = translateAccountError(err, fallback);
			return false;
		} finally {
			this.saving = false;
		}
	}

	/** Refreshes without the loading skeleton: the table stays visible while
	 *  a confirmed mutation lands. */
	private async reload(): Promise<void> {
		try {
			this.apply(await get<AccountsResponse>('/api/v1/admin/accounts'));
		} catch (err) {
			this.saveError = translateAccountError(err, m['admin.accounts.loadError']);
		}
	}

	private apply(response: AccountsResponse): void {
		this.accounts = response.accounts;
		this.max = response.max;
	}
}

const ADMIN_ACCOUNTS_CONTEXT_KEY = Symbol('admin-accounts');

export function setAdminAccountsContext(): AdminAccountsStore {
	const store = new AdminAccountsStore();
	setContext(ADMIN_ACCOUNTS_CONTEXT_KEY, store);
	return store;
}

export function getAdminAccountsContext(): AdminAccountsStore {
	return getContext<AdminAccountsStore>(ADMIN_ACCOUNTS_CONTEXT_KEY);
}
