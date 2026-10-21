import { test, expect } from '@playwright/test';

// Ticket 14: the shared Dialog is a native modal - the page behind neither
// scrolls nor takes focus, Escape closes it, focus returns to the trigger.
test('a dialog is a real modal: no background scroll, Escape closes, focus returns', async ({ page }) => {
	await page.request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	await page.setViewportSize({ width: 1280, height: 600 });
	await page.goto('/vms/default/100');
	await page.getByTestId('vm-tab-configuration').click();

	const trigger = page.getByTestId('vm-action-delete');
	await trigger.click();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();
	// Native modal: top layer, so nothing outside it is in the accessibility tree.
	expect(await dialog.evaluate((el) => el.tagName === 'DIALOG' && (el as HTMLDialogElement).matches(':modal'))).toBe(true);

	const before = await page.evaluate(() => window.scrollY);
	await page.mouse.move(20, 300);
	await page.mouse.wheel(0, 800);
	await page.waitForTimeout(200);
	expect(await page.evaluate(() => window.scrollY)).toBe(before);

	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await expect(trigger).toBeFocused();
	expect(await page.evaluate(() => document.documentElement.style.overflow)).toBe('');
});
