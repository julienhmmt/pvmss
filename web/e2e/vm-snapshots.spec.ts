import { test, expect, type APIRequestContext } from '@playwright/test';

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice' } });
	expect(response.status()).toBe(200);
}

test.describe('T09 VM snapshots', () => {
	test.describe.configure({ mode: 'serial' });

	test('creates, restores, and deletes a snapshot through the task tray', async ({ page }) => {
		await signInAlice(page.request);
		const name = `snapshot-e2e-${Date.now()}`;
		await page.goto('/vms/default/102');
		await page.getByTestId('vm-tab-snapshots').click();
		await expect(page.getByTestId('snapshot-counter')).toHaveText('0/5');

		await page.getByTestId('snapshot-create-open').click();
		await page.getByTestId('snapshot-name').fill(name);
		await page.getByTestId('snapshot-description').fill('snapshot e2e');
		await page.getByTestId('snapshot-create-confirm').click();
		await expect(page.getByText(`Snapshot "${name}" created`)).toBeVisible({ timeout: 20000 });
		await expect(page.getByTestId('snapshot-row')).toContainText(name);

		await page.getByTestId('snapshot-rollback-open').click();
		await expect(page.getByText(/everything captured after this snapshot is lost/i)).toBeVisible();
		await page.getByTestId('snapshot-rollback-confirm').click();
		await expect(page.getByText(`VM "${name}" rolled back`)).toBeVisible({ timeout: 20000 });
		await expect(page.getByTestId('vm-status')).toContainText('stopped');

		await page.getByTestId('snapshot-delete-open').click();
		await page.getByTestId('snapshot-delete-confirm').click();
		await expect(page.getByText(`Snapshot "${name}" deleted`)).toBeVisible({ timeout: 20000 });
		await expect(page.getByTestId('snapshot-empty')).toBeVisible();
	});
});
