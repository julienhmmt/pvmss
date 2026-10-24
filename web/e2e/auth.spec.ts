import { test, expect } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

test.describe('T02 authentication', () => {
	test('signs in with the fake PVE account and ends the browser session', async ({ page }) => {
		await page.goto('/login');
		// US3: the sign-in route has no signed-in sidebar landmark.
		await expect(page.getByRole('complementary')).toHaveCount(0);
		await expect(page.getByTestId('app-sidebar')).toHaveCount(0);

		await page.locator('input[autocomplete="username"]').fill('alice');
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-alice');
		await page.locator('#login-cluster').selectOption('default');
		await page.locator('button[type="submit"]').click();
		// A signed-in pool user lands on the machine list (calm workspace).
		await expect(page).toHaveURL(/\/vms$/);

		const me = await page.request.get('/api/v1/auth/me');
		expect(me.status()).toBe(200);
		// toMatchObject, not toEqual: /auth/me also carries the display name and
		// cluster display name, which this test does not assert on.
		expect(await me.json()).toMatchObject({ username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: 'default' });

		const logout = await page.request.post('/api/v1/auth/logout', {
			headers: await csrfHeaders(page.request)
		});
		expect(logout.status()).toBe(204);
		const signedOut = await page.request.get('/api/v1/auth/me');
		expect(signedOut.status()).toBe(401);
	});

	test('redirects the local administrator to the admin dashboard', async ({ page }) => {
		await page.goto('/login');
		await page.getByRole('tab', { name: /administrat/i }).click();
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-e2e-admin');
		await page.locator('button[type="submit"]').click();
		await expect(page).toHaveURL(/\/admin$/);
	});

	test('signs in as the Proxmox cluster administrator (PVEAdmin) and lands on /admin', async ({ page }) => {
		await page.goto('/login');
		await page.locator('input[autocomplete="username"]').fill('admin');
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-admin');
		await page.locator('#login-cluster').selectOption('default');
		await page.locator('button[type="submit"]').click();
		await expect(page).toHaveURL(/\/admin$/);

		const me = await page.request.get('/api/v1/auth/me');
		expect(me.status()).toBe(200);
		expect(await me.json()).toMatchObject({
			username: 'admin@pve',
			isAdmin: true,
			isOwner: false,
			provider: 'pve',
			cluster: 'default'
		});
	});

	test('owner creates and deletes a local administrator account from Settings > Accounts', async ({ page, context }) => {
		const login = await page.request.post('/api/v1/auth/admin-login', {
			data: { password: 'pvmss-e2e-admin' }
		});
		expect(login.status()).toBe(200);

		const created = await page.request.post('/api/v1/admin/accounts', {
			headers: await csrfHeaders(page.request),
			data: { username: 'e2e-ops', password: 'e2e-ops-password' }
		});
		expect(created.status()).toBe(201);
		const account = (await created.json()) as { id: number; username: string };
		expect(account.username).toBe('e2e-ops');

		try {
			await context.clearCookies();
			await page.goto('/login');
			await page.getByRole('tab', { name: /administrat/i }).click();
			await page.locator('input[autocomplete="username"]').fill('e2e-ops');
			await page.locator('input[autocomplete="current-password"]').fill('e2e-ops-password');
			await page.locator('button[type="submit"]').click();
			await expect(page).toHaveURL(/\/admin$/);

			// Non-owner local admin gets 403 on accounts API.
			const forbidden = await page.request.get('/api/v1/admin/accounts');
			expect(forbidden.status()).toBe(403);
		} finally {
			await page.request.post('/api/v1/auth/admin-login', {
				data: { password: 'pvmss-e2e-admin' }
			});
			const del = await page.request.delete(`/api/v1/admin/accounts/${account.id}`, {
				headers: await csrfHeaders(page.request)
			});
			expect(del.status()).toBe(204);
		}
	});

	test('the sign-in gate returns the user to the page they asked for', async ({ page }) => {
		await page.goto('/vms?cluster=default');
		await page.getByRole('button', { name: /sign in|se connecter/i }).click();
		await expect(page).toHaveURL(/\/login\?next=%2Fvms%3Fcluster%3Ddefault$/);

		await page.locator('input[autocomplete="username"]').fill('alice');
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-alice');
		await page.locator('#login-cluster').selectOption('default');
		await page.locator('button[type="submit"]').click();
		await expect(page).toHaveURL(/\/vms\?cluster=default$/);

		// Signed in, /login is not a page: it sends the user on.
		await page.goto('/login');
		await expect(page).toHaveURL(/\/vms$/);
	});

	test('a failed sign-in moves focus to the error and links the fields to it', async ({ page }) => {
		await page.goto('/login');
		await page.locator('input[autocomplete="username"]').fill('alice');
		await page.locator('input[autocomplete="current-password"]').fill('wrong-password');
		await page.locator('#login-cluster').selectOption('default');
		await page.locator('button[type="submit"]').click();

		await expect(page.locator('#login-error')).toBeFocused();
		await expect(page.locator('input[autocomplete="current-password"]')).toHaveAttribute('aria-describedby', /login-error/);
	});

	test('an expired session sends the user to sign in once and back', async ({ page, context }) => {
		await page.goto('/login');
		await page.locator('input[autocomplete="username"]').fill('alice');
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-alice');
		await page.locator('#login-cluster').selectOption('default');
		await page.locator('button[type="submit"]').click();
		await expect(page).toHaveURL(/\/vms$/);

		// The server forgets the session; the next in-app call gets a 401.
		await context.clearCookies();
		await page.getByTestId('app-sidebar').getByRole('link', { name: /activity|activité/i }).first().click();

		await expect(page).toHaveURL(/\/login\?next=%2Factivity$/);
		await expect(page.getByText(/session expired|session a expiré/i)).toHaveCount(1);

		await page.locator('input[autocomplete="username"]').fill('alice');
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-alice');
		await page.locator('#login-cluster').selectOption('default');
		await page.locator('button[type="submit"]').click();
		await expect(page).toHaveURL(/\/activity$/);
	});
});
