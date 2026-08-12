import { test, expect } from '@playwright/test';

test.describe('T02 authentication', () => {
	test('signs in with the fake PVE account and ends the browser session', async ({ page }) => {
		await page.goto('/login');
		await page.getByLabel('Username').fill('alice');
		await page.getByLabel('Password').fill('pvmss-alice');
		await page.locator('#login-cluster').selectOption('default');
		await page.getByRole('button', { name: 'Sign in' }).click();
		await expect(page).toHaveURL(/\/nodes$/);

		const me = await page.request.get('/api/v1/auth/me');
		expect(me.status()).toBe(200);
		expect(await me.json()).toEqual({ username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: 'default' });

		const logout = await page.request.post('/api/v1/auth/logout');
		expect(logout.status()).toBe(204);
		const signedOut = await page.request.get('/api/v1/auth/me');
		expect(signedOut.status()).toBe(401);
	});
});
