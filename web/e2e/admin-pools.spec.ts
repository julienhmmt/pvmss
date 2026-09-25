import { expect, test } from '@playwright/test';

test.describe('T13 admin pools', () => {
	test.describe.configure({ mode: 'serial' });

	test('creates, searches, and deletes a pool as an admin', async ({ page }) => {
		const login = await page.request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
		expect(login.status()).toBe(200);

		await page.goto('/admin/pools');
		await expect(page.getByRole('heading', { name: 'Pools' })).toBeVisible();
		// Two "New pool" buttons exist (toolbar + empty state).
		await page.getByRole('button', { name: 'New pool' }).first().click();
		await page.getByLabel('Name').fill('e2eteam');
		// No password field: PVMSS generates the pool credentials itself and
		// shows them once in a banner.
		await page.getByRole('button', { name: 'Create pool' }).click();
		await expect(page.getByRole('row', { name: /e2eteam/ })).toBeVisible();

		// Dismiss the one-time credentials banner so it cannot intercept clicks.
		const dismissCredentials = page.getByRole('button', { name: /saved the credentials/i });
		if (await dismissCredentials.isVisible().catch(() => false)) {
			await dismissCredentials.click();
		}

		await page.getByLabel('Search pools').fill('E2E');
		await expect(page.getByRole('row', { name: /e2eteam/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /alice/ })).toHaveCount(0);

		await page.getByRole('row', { name: /e2eteam/ }).getByRole('button', { name: 'Delete' }).click();
		const deleteDialog = page.getByRole('dialog', { name: 'Delete pool' });
		await expect(deleteDialog).toBeVisible();
		// Scope to the dialog: the row also has a button whose accessible name
		// starts with "Delete pool".
		await deleteDialog.getByRole('button', { name: 'Delete pool' }).click();
		await expect(page.getByRole('row', { name: /e2eteam/ })).toHaveCount(0);
	});
});
