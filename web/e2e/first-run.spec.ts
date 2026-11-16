import { test, expect, type Page } from '@playwright/test';

// Both fake pool users own machines, so the list response is stubbed to the
// empty-workspace shape the server sends a brand-new pool user.
async function signInWithEmptyWorkspace(page: Page, allowed: number): Promise<void> {
	const response = await page.request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
	await page.route('**/api/v1/vms?*', (route) =>
		route.fulfill({
			json: {
				items: [],
				total: 0,
				page: 1,
				pageSize: 20,
				availableNodes: [],
				availablePools: [],
				emptyReason: 'no_vms_owned',
				quota: { used: 0, allowed }
			}
		})
	);
}

test.describe('Pool user first run', () => {
	test('explains the allowance and leads to the wizard and the guide', async ({ page }) => {
		await signInWithEmptyWorkspace(page, 3);
		await page.goto('/vms?cluster=default');

		await expect(page.getByTestId('vm-first-run-allowance')).toContainText('You can run up to 3 machines');
		await expect(page.getByTestId('vm-quota-full')).toBeHidden();

		await page.getByTestId('vm-first-run-guide').click();
		await expect(page).toHaveURL(/\/docs\/getting-started/);

		await page.goBack();
		await page.getByTestId('vm-empty-create').click();
		await expect(page).toHaveURL(/\/vms\/create/);
	});

	test('a zero allowance offers a request instead of a create', async ({ page }) => {
		await signInWithEmptyWorkspace(page, 0);
		await page.goto('/vms?cluster=default');

		await expect(page.getByTestId('vm-first-run-zero')).toBeVisible();
		await expect(page.getByTestId('vm-empty-create')).toBeHidden();
		await expect(page.getByTestId('vm-quota-full')).toBeHidden();
		await expect(page.getByRole('button', { name: /request/i })).toBeVisible();
	});
});
