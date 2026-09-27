import { expect, test } from '@playwright/test';

test.describe('PVMSS user administration', () => {
	test.describe.configure({ mode: 'serial' });

	test('creates, searches, and deletes a PVMSS user as an admin', async ({ page }) => {
		const login = await page.request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
		expect(login.status()).toBe(200);

		await page.goto('/admin/pools');
		await expect(page.getByRole('heading', { name: 'PVMSS users' })).toBeVisible();
		await page.getByRole('button', { name: 'Create PVMSS user' }).first().click();
		await page.getByLabel('User name').fill('e2eteam');
		// No password field: PVMSS generates the sign-in credentials and shows them once.
		await page.getByRole('button', { name: 'Create user' }).click();
		await expect(page.getByRole('row', { name: /pvmss-e2eteam@pve/ })).toBeVisible();
		await expect(page.getByText('PVMSS user created')).toBeVisible();

		const dismissCredentials = page.getByRole('button', { name: /shared these credentials/i });
		if (await dismissCredentials.isVisible().catch(() => false)) {
			await dismissCredentials.click();
		}

		await page.getByLabel('Search PVMSS users').fill('E2E');
		await expect(page.getByRole('row', { name: /pvmss-e2eteam@pve/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /alice/ })).toHaveCount(0);

		const userRow = page.getByRole('row', { name: /pvmss-e2eteam@pve/ });
		await userRow.getByRole('button', { name: /Delete PVMSS user/ }).click();
		const deleteDialog = page.getByRole('dialog', { name: 'Delete PVMSS user?' });
		await expect(deleteDialog).toBeVisible();
		await deleteDialog.getByRole('button', { name: 'Delete user' }).click();
		await expect(page.getByRole('row', { name: /pvmss-e2eteam@pve/ })).toHaveCount(0);
	});
});
