import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

const ADMIN_PASSWORD = 'pvmss-e2e-admin';

async function signInAdmin(request: APIRequestContext, page: Page): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: ADMIN_PASSWORD } });
	expect(response.status()).toBe(200);
	await page.context().addCookies((await request.storageState()).cookies);
}

/**
 * Read-only on purpose: the fake dataset is shared by the whole run and other
 * specs assert its pristine VM statuses, so this spec never powers a machine
 * on or off.
 */
test.describe('Admin all machines', () => {
	test('lists every pool and opens the owner detail view from a row', async ({ page, request }) => {
		await signInAdmin(request, page);

		await page.goto('/admin/vms');
		await expect(page.getByRole('heading', { name: 'All machines' })).toBeVisible();

		const rows = page.getByTestId('admin-vm-row');
		await expect(rows.first()).toBeVisible();

		// Scope is admin-wide: VMs from more than one pool are listed.
		await expect.poll(async () => {
			const pools = await page.getByTestId('admin-vm-pool').allTextContents();
			return new Set(pools.map((pool) => pool.trim())).size;
		}).toBeGreaterThan(1);

		// Quick actions are present on the row: a running machine has a console
		// link, a stopped one a start button.
		await expect(
			page.locator('[data-testid="admin-vm-start"], [data-testid="admin-vm-console"]').first()
		).toBeVisible();

		// The pool filter narrows the list server-side and keeps the facet.
		await page.getByLabel('Filter by pool', { exact: true }).selectOption('pool-bob');
		await expect.poll(async () => {
			const pools = await page.getByTestId('admin-vm-pool').allTextContents();
			return pools.length > 0 && pools.every((pool) => pool.trim() === 'pool-bob');
		}).toBe(true);

		// A row (or its name) opens the same detail view the owner gets, under
		// the admin route, and Back returns to the admin collection.
		await page.getByTestId('admin-vm-row-link').first().click();
		await expect(page).toHaveURL(/\/admin\/vms\/[^/]+\/\d+$/);
		await expect(page.getByTestId('vm-detail-header')).toBeVisible();

		await page.getByTestId('page-back-link').click();
		await expect(page).toHaveURL(/\/admin\/vms$/);
		await expect(page.getByRole('heading', { name: 'All machines' })).toBeVisible();
	});
});
