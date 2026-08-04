import { test, expect, type APIRequestContext } from '@playwright/test';

async function signIn(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice' }
	});
	expect(response.status()).toBe(200);
}

test.describe('T01 fake cluster', () => {
	test('shows three nodes with one offline, stable across reload', async ({ page }) => {
		await signIn(page.request);
		await page.goto('/nodes');

		const rows = page.locator('tbody tr');
		await expect(rows).toHaveCount(3);
		await expect(page.getByText('pve-node-01')).toBeVisible();
		await expect(page.getByText('pve-node-02')).toBeVisible();
		await expect(page.getByText('pve-node-03')).toBeVisible();
		await expect(page.locator('tbody tr', { hasText: 'pve-node-03' }).getByText('offline')).toBeVisible();

		const firstLoadNames = await rows.allTextContents();
		await page.reload();
		expect(await rows.allTextContents()).toEqual(firstLoadNames);
	});

	test('cluster nodes API requires an authenticated fake user', async ({ request }) => {
		expect((await request.get('/api/v1/cluster/nodes')).status()).toBe(401);
		await signIn(request);
		const response = await request.get('/api/v1/cluster/nodes');
		expect(response.status()).toBe(200);
		const body = await response.json();
		expect(body.nodes).toHaveLength(3);
		expect(body.nodes[0]).toMatchObject({ name: 'pve-node-01', status: 'online' });
	});
});
