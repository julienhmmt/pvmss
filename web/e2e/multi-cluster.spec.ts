import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

async function signIn(request: APIRequestContext, username: string, password: string, cluster: string): Promise<void> {
	const response = await request.post('/api/v1/auth/login', { data: { username, password, cluster } });
	expect(response.status()).toBe(200);
}

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

function vmRows(page: Page) {
	return page.locator('[data-testid="vm-row"]');
}

test.describe('T15 multi-cluster', () => {
	test('merges Alice VMs and keeps the composite VM identity', async ({ page }) => {
		await signIn(page.request, 'alice', 'pvmss-alice', 'default');
		await page.goto('/vms?pageSize=100');
		await expect(vmRows(page).first()).toBeVisible();
		expect(await vmRows(page).count()).toBeGreaterThanOrEqual(12);
		await expect(vmRows(page).locator('td:first-child').filter({ hasText: 'default' })).toHaveCount(7);
		await expect(vmRows(page).locator('td:first-child').filter({ hasText: 'secondary' })).toHaveCount(5);

		const secondaryVM = page.locator('tr', { hasText: 'secondary-web-02' });
		await expect(secondaryVM).toBeVisible();
		await expect(secondaryVM.getByRole('link')).toHaveAttribute('href', /\/vms\/secondary\/101$/);

		await page.locator('#vm-cluster-filter').selectOption('secondary');
		await expect(vmRows(page)).toHaveCount(5);
		await expect(vmRows(page).getByRole('link', { name: 'secondary-web-02' })).toHaveAttribute('href', /\/vms\/secondary\/101$/);
	});

	test('admin sees cluster health, required catalog selection, and can add a cluster', async ({ page }) => {
		await signInAdmin(page.request);
		const clustersResponse = await page.request.get('/api/v1/admin/clusters');
		expect(clustersResponse.status()).toBe(200);
		const clusters = (await clustersResponse.json()) as Array<{ name: string }>;
		expect(clusters.map((cluster) => cluster.name)).toEqual(expect.arrayContaining(['default', 'secondary', 'offline-demo']));
		expect((await page.request.get('/api/v1/admin/nodes')).status()).toBe(400);

		await page.goto('/admin/clusters');
		const existing = page.locator('tr', { hasText: 'e2e-tertiary' });
		if ((await existing.count()) === 0) {
			await page.getByRole('button', { name: 'Add cluster' }).click();
			await page.getByLabel('Name').fill('e2e-tertiary');
			await page.getByLabel('URL').fill('https://pve-e.invalid:8006/api2/json');
			await page.getByLabel('Token ID').fill('pvmss@pve!service');
			await page.getByLabel(/Token secret/).fill('e2e-service-secret');
			await page.getByRole('button', { name: 'Save' }).click();
		}
		const tertiary = page.locator('tr', { hasText: 'e2e-tertiary' });
		await expect(tertiary).toBeVisible();
		await expect(tertiary.getByText('untested')).toBeVisible();
		const oidcButton = tertiary.getByRole('button', { name: 'Enable OIDC' });
		if ((await oidcButton.count()) > 0) await oidcButton.click();
		await expect(tertiary.getByText('Enabled')).toBeVisible();
	});
});
