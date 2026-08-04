import { test, expect } from '@playwright/test';

test.describe('T02 API tokens', () => {
	// Bearer checks use the top-level `request` fixture, not `page.request` —
	// the latter shares the browser's session cookie, which would keep
	// authenticating these calls even after the bearer token is revoked.
	test('creates, uses, lists, and revokes a token from the profile page', async ({ page, request }) => {
		await page.goto('/login');
		await page.getByLabel('Username').fill('alice');
		await page.getByLabel('Password').fill('pvmss-alice');
		await page.getByRole('button', { name: 'Sign in' }).click();
		await expect(page).toHaveURL(/\/nodes$/);

		await page.goto('/profile/tokens');
		await page.getByLabel('Label').fill('automation');
		await page.getByRole('button', { name: 'Create token' }).click();

		const valueBlock = page.locator('code');
		await expect(valueBlock).toBeVisible();
		const token = ((await valueBlock.textContent()) ?? '').trim();
		expect(token).toMatch(/^pvmss_/);

		const bearerResponse = await request.get('/api/v1/cluster/nodes', {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(bearerResponse.status()).toBe(200);

		await expect(page.getByRole('row', { name: /automation/ })).toBeVisible();
		await page.getByRole('button', { name: 'Revoke' }).click();
		await expect(page.getByRole('row', { name: /automation/ })).toHaveCount(0);

		const revokedResponse = await request.get('/api/v1/cluster/nodes', {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(revokedResponse.status()).toBe(401);
	});
});
