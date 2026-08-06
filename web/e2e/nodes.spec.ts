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

test.describe('T03 inventory projection', () => {
	// These tests share one server-side manual-refresh guard (the guard clock
	// lives on the backend's single Worker/Projection, not per-test state).
	// Running them concurrently lets one test's refresh reset another's guard
	// window, so they must run serially against the shared server.
	test.describe.configure({ mode: 'serial' });

	test('each node shows a VM count column', async ({ page }) => {
		await signIn(page.request);
		await page.goto('/nodes');

		const vmCounts = page.locator('[data-testid="vm-count"]');
		await expect(vmCounts).toHaveCount(3);
		// The fake dataset has 12 VMs on pve-node-01, 10 on pve-node-02, 3 on pve-node-03.
		const counts = await vmCounts.allTextContents();
		expect(counts.map((c) => Number(c.trim()))).toContain(12);
	});

	test('last refreshed time is displayed', async ({ page }) => {
		await signIn(page.request);
		await page.goto('/nodes');

		const refreshedAt = page.locator('[data-testid="refreshed-at"] time');
		await expect(refreshedAt).toBeVisible();
		const firstTime = await refreshedAt.getAttribute('datetime');
		expect(firstTime).not.toBeNull();
	});

	test('manual refresh updates the displayed time', async ({ page }) => {
		await signIn(page.request);
		await page.goto('/nodes');

		const refreshedAt = page.locator('[data-testid="refreshed-at"] time');
		const refreshButton = page.locator('[data-testid="refresh-button"]');
		await expect(refreshedAt).toBeVisible();
		const firstTime = await refreshedAt.getAttribute('datetime');

		// The manual-refresh guard clock starts at server boot (the worker
		// runs one refresh immediately on startup — worker.go Run()), so a
		// click landing within that window is correctly guarded (429) and
		// leaves refreshedAt unchanged. Retry until the guard has cleared and
		// a click actually performs a refresh, instead of assuming the first
		// click always lands outside the boot guard window.
		await expect(async () => {
			if (await refreshButton.isEnabled()) {
				await refreshButton.click();
			}
			await expect(refreshedAt).not.toHaveAttribute('datetime', firstTime ?? '');
		}).toPass({ timeout: 10000 });
	});

	test('second immediate refresh is visibly guarded (disabled)', async ({ page }) => {
		await signIn(page.request);
		await page.goto('/nodes');

		const refreshButton = page.locator('[data-testid="refresh-button"]');
		await expect(refreshButton).toBeEnabled();

		await refreshButton.click();
		// After the first refresh, the guard should disable the button or show the wait state.
		await expect(refreshButton).toBeDisabled({ timeout: 5000 });
		await expect(page.locator('[data-testid="refresh-error"]')).toBeVisible();
	});

	test('the guard is temporary — waiting it out re-enables the button (quickstart step 6)', async ({ page }) => {
		await signIn(page.request);
		await page.goto('/nodes');

		const refreshButton = page.locator('[data-testid="refresh-button"]');
		await refreshButton.click();
		await expect(refreshButton).toBeDisabled({ timeout: 5000 });

		// Guard is configured to 2s for e2e (playwright.config.ts) — wait it out.
		await expect(refreshButton).toBeEnabled({ timeout: 5000 });
		await expect(page.locator('[data-testid="refresh-error"]')).toBeHidden();
	});
});
