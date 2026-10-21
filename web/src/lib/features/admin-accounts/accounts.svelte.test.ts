import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminAccountsStore, type LocalAccount } from './accounts.svelte';
import { m } from '#lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function account(overrides: Partial<LocalAccount> = {}): LocalAccount {
	return {
		id: 1,
		username: 'albert',
		disabled: false,
		failedAttempts: 0,
		createdAt: '2026-10-01T10:00:00Z',
		updatedAt: '2026-10-01T10:00:00Z',
		createdBy: 'admin',
		...overrides
	};
}

describe('AdminAccountsStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads the accounts and the cap', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValueOnce(
				jsonResponse(200, { accounts: [account(), account({ id: 2, username: 'berta' })], max: 5, count: 2 })
			)
		);
		const store = new AdminAccountsStore();
		await store.load();
		expect(store.accounts.map((entry) => entry.username)).toEqual(['albert', 'berta']);
		expect(store.max).toBe(5);
		expect(store.count).toBe(2);
		expect(store.error).toBeNull();
	});

	it('reports the cap as reached only when the limit is known', () => {
		const store = new AdminAccountsStore();
		expect(store.atLimit).toBe(false);
		store.max = 1;
		store.accounts = [account()];
		expect(store.atLimit).toBe(true);
	});

	it('creates an account then reloads the list', async () => {
		const created = account({ id: 3, username: 'carla' });
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(201, created))
			.mockResolvedValueOnce(jsonResponse(200, { accounts: [created], max: 5, count: 1 }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new AdminAccountsStore();
		const ok = await store.create('carla', 'pvmss-carla');

		expect(ok).toBe(true);
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(init?.method).toBe('POST');
		expect(JSON.parse(init?.body as string)).toEqual({ username: 'carla', password: 'pvmss-carla' });
		expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/v1/admin/accounts');
		expect(store.accounts).toHaveLength(1);
		expect(store.saveError).toBeNull();
	});

	it('translates the account limit error', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValueOnce(
				jsonResponse(409, { code: 'account_limit_reached', message: 'local administrator limit reached' })
			)
		);
		const store = new AdminAccountsStore();
		const ok = await store.create('carla', 'pvmss-carla');
		expect(ok).toBe(false);
		expect(store.saveError).toBe(m['admin.accounts.errorLimitReached']());
	});

	it('translates an unknown account id on delete', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValueOnce(jsonResponse(404, { code: 'not_found', message: 'local account not found' }))
		);
		const store = new AdminAccountsStore();
		const ok = await store.remove(9);
		expect(ok).toBe(false);
		expect(store.saveError).toBe(m['admin.accounts.errorNotFound']());
	});

	it('resets a password with a PUT and reloads', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(new Response(null, { status: 204 }))
			.mockResolvedValueOnce(jsonResponse(200, { accounts: [account()], max: 5, count: 1 }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new AdminAccountsStore();
		const ok = await store.resetPassword(1, 'pvmss-newpass');

		expect(ok).toBe(true);
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(init?.method).toBe('PUT');
		expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/admin/accounts/1/password');
		expect(JSON.parse(init?.body as string)).toEqual({ password: 'pvmss-newpass' });
	});

	it('toggles an account and sends the target state', async () => {
		const disabled = account({ disabled: true });
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(new Response(null, { status: 204 }))
			.mockResolvedValueOnce(jsonResponse(200, { accounts: [disabled], max: 5, count: 1 }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new AdminAccountsStore();
		const ok = await store.setDisabled(1, true);

		expect(ok).toBe(true);
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/admin/accounts/1/toggle');
		expect(JSON.parse(init?.body as string)).toEqual({ disabled: true });
		expect(store.accounts[0]?.disabled).toBe(true);
	});

	it('deletes an account with DELETE and reloads', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(new Response(null, { status: 204 }))
			.mockResolvedValueOnce(jsonResponse(200, { accounts: [], max: 5, count: 0 }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new AdminAccountsStore();
		const ok = await store.remove(1);

		expect(ok).toBe(true);
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(init?.method).toBe('DELETE');
		expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/admin/accounts/1');
		expect(store.accounts).toEqual([]);
	});
});
