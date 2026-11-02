import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

async function signInAdmin(request: APIRequestContext, page: Page): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
	await page.context().addCookies((await request.storageState()).cookies);
}

test.describe('Tooltip', () => {
	test('stays inside the viewport at the top edge and closes on Escape', async ({ page, request }) => {
		await signInAdmin(request, page);
		await page.goto('/admin/pools');

		const trigger = page.locator('[data-tooltip-trigger]').first();
		await expect(trigger).toBeVisible();

		// Pin the trigger to the very top of the viewport: a top-positioned
		// bubble would overflow without collision handling (the old hand-rolled
		// placement bug this regression guards).
		await trigger.evaluate((el) => el.scrollIntoView({ block: 'start' }));
		await trigger.dispatchEvent('pointerenter');

		const bubble = page.locator('[role="tooltip"]');
		await expect(bubble).toBeVisible({ timeout: 3000 });

		const box = await bubble.boundingBox();
		expect(box).not.toBeNull();
		expect(box!.y).toBeGreaterThanOrEqual(0);
		expect(box!.x).toBeGreaterThanOrEqual(0);

		await page.keyboard.press('Escape');
		await expect(bubble).toBeHidden();
	});
});
