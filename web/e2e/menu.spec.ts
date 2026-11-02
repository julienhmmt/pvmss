import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

async function signInAlice(request: APIRequestContext, page: Page): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
	await page.context().addCookies((await request.storageState()).cookies);
}

test.describe('Overflow menu', () => {
	test('the compact action bar folds secondary actions into a menu', async ({ page, request }) => {
		await signInAlice(request, page);
		// web-01 (vmid 100) is running in the fake dataset, so several power
		// actions apply: the lead one stays a button, the rest fold away.
		await page.goto('/vms/default/100/console');

		const bar = page.locator('[data-testid="vm-action-bar"]');
		await expect(bar).toBeVisible();
		const menuTrigger = bar.locator('[data-testid="vm-action-menu"]');
		await expect(menuTrigger).toBeVisible();

		await menuTrigger.click();
		const menu = page.locator('[role="menu"]');
		await expect(menu).toBeVisible();
		// stop / reboot / reset / pause - every item announced as a menuitem.
		await expect(menu.locator('[role="menuitem"]')).toHaveCount(4);
		await expect(menu.getByTestId('vm-menu-stop')).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(menu).toBeHidden();
		await expect(menuTrigger).toBeFocused();
	});
});
