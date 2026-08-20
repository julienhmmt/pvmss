import { test, expect } from '@playwright/test';

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
		await expect(page).toHaveURL(/\/nodes$/);

		const me = await page.request.get('/api/v1/auth/me');
		expect(me.status()).toBe(200);
		expect(await me.json()).toEqual({ username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: 'default' });

		const logout = await page.request.post('/api/v1/auth/logout');
		expect(logout.status()).toBe(204);
		const signedOut = await page.request.get('/api/v1/auth/me');
		expect(signedOut.status()).toBe(401);
	});

	test('redirects the local administrator to the admin dashboard', async ({ page }) => {
		await page.goto('/login');
		await page.locator('input[type="radio"][value="local"]').check();
		await page.locator('input[autocomplete="current-password"]').fill('pvmss-e2e-admin');
		await page.locator('button[type="submit"]').click();
		await expect(page).toHaveURL(/\/admin$/);
	});
});
