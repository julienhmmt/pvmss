import { test, expect } from '@playwright/test';

test.describe('T01 fake cluster', () => {
	test('shows three nodes with one offline, stable across reload', async ({ page }) => {
		await page.goto('/nodes');

		const rows = page.locator('tbody tr');
		await expect(rows).toHaveCount(3);

		await expect(page.getByText('pve-node-01')).toBeVisible();
		await expect(page.getByText('pve-node-02')).toBeVisible();
		await expect(page.getByText('pve-node-03')).toBeVisible();

		const offlineRow = page.locator('tbody tr', { hasText: 'pve-node-03' });
		await expect(offlineRow.getByText('offline')).toBeVisible();

		const firstLoadNames = await rows.allTextContents();
		await page.reload();
		const secondLoadNames = await rows.allTextContents();
		expect(secondLoadNames).toEqual(firstLoadNames);
	});

	test('cluster nodes API responds with the fake dataset', async ({ request }) => {
		const response = await request.get('/api/v1/cluster/nodes');
		expect(response.status()).toBe(200);
		const body = await response.json();
		expect(body.nodes).toHaveLength(3);
		expect(body.nodes[0]).toMatchObject({ name: 'pve-node-01', status: 'online' });
	});
});
