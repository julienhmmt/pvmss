import { test, expect } from '@playwright/test';

test.describe('T00 smoke', () => {
	test('shell renders the placeholder title', async ({ page }) => {
		await page.goto('/');
		// T19: the homepage now uses Paraglide messages. The default locale is
		// French, so the title appears in French. "PVMSS v0.4" is locale-
		// independent (a version string).
		await expect(page.locator('text=PVMSS v0.4')).toBeVisible();
	});

	test('health endpoint returns healthy', async ({ request }) => {
		const response = await request.get('/health');
		expect(response.status()).toBe(200);
		const body = await response.json();
		expect(body).toMatchObject({
			status: 'healthy',
			checks: { database: { status: 'healthy' } },
		});
		expect(body.timestamp).toBeDefined();
	});

	test('unknown client route falls back to the shell', async ({ request }) => {
		const response = await request.get('/not-a-real-route');
		expect(response.status()).toBe(200);
		const body = await response.text();
		expect(body).toContain('<title>PVMSS</title>');
		expect(body).toContain('app');
	});
});
