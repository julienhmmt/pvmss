import { test, expect, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

async function signIn(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

test.describe('T01 fake cluster', () => {
	// The user-facing /nodes page was dropped; the admin catalog node table
	// (/admin/nodes) is the surviving node list.
	test('shows three nodes with one offline, stable across reload', async ({ page }) => {
		await signInAdmin(page.request);
		await page.goto('/admin/nodes');

		const rows = page.locator('[data-testid="node-row"]');
		await expect(rows).toHaveCount(3);
		await expect(page.getByText('pve-node-01')).toBeVisible();
		await expect(page.getByText('pve-node-02')).toBeVisible();
		await expect(page.getByText('pve-node-03')).toBeVisible();
		await expect(page.locator('[data-testid="node-row"]', { hasText: 'pve-node-03' }).getByText('Offline')).toBeVisible();

		const firstLoadNames = await rows.allTextContents();
		await page.reload();

		// The table renders skeleton rows before its data arrives, so wait for
		// the real rows instead of reading straight after the reload.
		await expect(rows).toHaveCount(3);
		await expect(page.locator('[data-testid="node-row"]', { hasText: 'pve-node-03' }).getByText('Offline')).toBeVisible();
		expect(await rows.allTextContents()).toEqual(firstLoadNames);
	});

	test('cluster nodes API requires an authenticated fake user', async ({ request }) => {
		expect((await request.get('/api/v1/cluster/nodes')).status()).toBe(401);
		await signIn(request);
		const response = await request.get('/api/v1/cluster/nodes');
		expect(response.status()).toBe(200);
		const body = (await response.json()) as { nodes: Array<{ name: string; status: string }> };
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
		await signInAdmin(page.request);
		await page.goto('/admin/nodes');

		const vmCounts = page.locator('[data-testid="node-row"] td[data-label="VMs"]');
		await expect(vmCounts).toHaveCount(3);
		// The fake dataset has 12 VMs on pve-node-01, 10 on pve-node-02, 3 on pve-node-03.
		const counts = await vmCounts.allTextContents();
		expect(counts.map((c) => Number(c.trim()))).toContain(12);
	});

	// The refresh button and refreshed-at stamp live on /admin/policy/nodes.
	test('last refreshed time is displayed', async ({ page }) => {
		await signInAdmin(page.request);
		await page.goto('/admin/policy/nodes');

		const refreshedAt = page.locator('[data-testid="refreshed-at"] time');
		await expect(refreshedAt).toBeVisible();
		const firstTime = await refreshedAt.getAttribute('datetime');
		expect(firstTime).not.toBeNull();
	});

	test('manual refresh updates the displayed time', async ({ page }) => {
		await signInAdmin(page.request);
		await page.goto('/admin/policy/nodes');

		const refreshedAt = page.locator('[data-testid="refreshed-at"] time');
		const refreshButton = page.locator('[data-testid="refresh-button"]');
		await expect(refreshedAt).toBeVisible();
		const firstTime = await refreshedAt.getAttribute('datetime');

		// An admin session refreshes every cluster at once: the endpoint
		// always answers 202 and marks guarded clusters "too_soon" instead of
		// failing the call, so a click inside the 2s window is silently
		// dropped for "default". Wait for the shown timestamp to age past the
		// guard so the click is accepted for certain.
		if (firstTime !== null) {
			const age = Date.now() - Date.parse(firstTime);
			if (age < 2500) await page.waitForTimeout(2600 - age);
		}
		await refreshButton.click();
		await expect(refreshedAt).not.toHaveAttribute('datetime', firstTime ?? '', { timeout: 20000 });
	});

	// The 429 guard is only reachable for a cluster-scoped session: an admin
	// session fans the refresh out to every cluster and gets 202 with
	// per-cluster "accepted"/"too_soon" results, so no admin page can show
	// the wait state the old /nodes page had. The guard contract is asserted
	// at the API level with alice's scoped session instead.
	test('a refresh inside the window is refused, then accepted once the guard clears', async ({ request }) => {
		await signIn(request);
		const headers = await csrfHeaders(request);

		// A previous spec's refresh may still sit inside the 2s window, so
		// retry until the server accepts one and the guard clock is armed by
		// a known moment.
		await expect(async () => {
			const response = await request.post('/api/v1/cluster/refresh', { headers });
			expect(response.status()).toBe(202);
		}).toPass({ timeout: 10000 });

		// The very next call lands inside the window: 429 with a countdown.
		const guarded = await request.post('/api/v1/cluster/refresh', { headers });
		expect(guarded.status()).toBe(429);
		expect(((await guarded.json()) as { code: string }).code).toBe('refresh_too_soon');

		// The guard is configured to 2s for e2e (playwright.config.ts) - wait
		// it out and the next call is accepted again (quickstart step 6).
		await expect(async () => {
			const response = await request.post('/api/v1/cluster/refresh', { headers });
			expect(response.status()).toBe(202);
		}).toPass({ timeout: 5000 });
	});
});
