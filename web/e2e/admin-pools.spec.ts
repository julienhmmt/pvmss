import { expect, test } from '@playwright/test';

test.describe('T13 admin pools', () => {
	test.describe.configure({ mode: 'serial' });

	test('creates, searches, and deletes a pool as an admin', async ({ page }) => {
		const login = await page.request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
		expect(login.status()).toBe(200);

		await page.goto('/admin/pools');
		await expect(page.getByRole('heading', { name: 'Pools' })).toBeVisible();
		await page.getByRole('button', { name: 'New pool' }).click();
		await page.getByLabel('Name').fill('e2eteam');
		await page.getByLabel('Initial password').fill('S0meLongPW!');
		await page.getByRole('button', { name: 'Create pool' }).click();
		await expect(page.getByRole('row', { name: /e2eteam/ })).toBeVisible();

		await page.getByLabel('Search pools').fill('E2E');
		await expect(page.getByRole('row', { name: /e2eteam/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /alice/ })).toHaveCount(0);

		await page.getByRole('row', { name: /e2eteam/ }).getByRole('button', { name: 'Delete' }).click();
		await expect(page.getByRole('dialog', { name: 'Delete pool' })).toBeVisible();
		await page.getByRole('button', { name: 'Delete pool' }).click();
		await expect(page.getByRole('row', { name: /e2eteam/ })).toHaveCount(0);
	});
});
