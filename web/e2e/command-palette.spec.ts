import { test, expect, type Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Ticket 06: Cmd/Ctrl+K jumps to a page or a machine, keyboard only.
async function signInAlice(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	expect(response.status()).toBe(200);
}

test.describe('T06 command palette', () => {
	test('opens over a focused input, filters, and Enter navigates', async ({ page }) => {
		await signInAlice(page);
		await page.goto('/vms?cluster=default');
		const search = page.getByTestId('vm-search');
		await search.focus();

		await page.keyboard.press('ControlOrMeta+k');
		const input = page.getByTestId('palette-input');
		await expect(input).toBeFocused();
		await expect(search).toHaveValue('');

		// A pool user is never offered admin pages.
		await expect(page.getByTestId('palette-option').filter({ hasText: /Node capacity|Capacité des nœuds/ })).toHaveCount(0);

		await input.fill('activ');
		await expect(page.getByRole('dialog').getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
		await page.keyboard.press('Enter');
		await expect(page).toHaveURL(/\/activity$/);
		await expect(page.getByRole('dialog')).toHaveCount(0);
	});

	test('finds a machine by name and opens it', async ({ page }) => {
		await signInAlice(page);
		await page.goto('/activity');
		await expect(page.getByTestId('activity-recent')).toBeVisible();
		await page.keyboard.press('ControlOrMeta+k');
		await page.getByTestId('palette-input').fill('web-01');
		// No page is called web-01, so the machine is the first and active option.
		await expect(page.getByRole('dialog').getByRole('option').first()).toContainText('web-01');
		await page.keyboard.press('Enter');
		await expect(page).toHaveURL(/\/vms\/default\/100$/);
	});

	test('Escape closes and returns focus to where the user was', async ({ page }) => {
		await signInAlice(page);
		await page.goto('/vms?cluster=default');
		const search = page.getByTestId('vm-search');
		await search.focus();
		await page.keyboard.press('ControlOrMeta+k');
		await expect(page.getByRole('dialog')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog')).toHaveCount(0);
		await expect(search).toBeFocused();
	});

	test('is reachable from a button on a phone', async ({ page }) => {
		await signInAlice(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/vms?cluster=default');
		await page.getByTestId('palette-open-mobile').click();
		await expect(page.getByTestId('palette-input')).toBeFocused();
	});

	test('passes axe with results and with nothing matching', async ({ page }) => {
		await signInAlice(page);
		await page.goto('/vms?cluster=default');
		await page.getByTestId('vm-search').focus();
		await page.keyboard.press('ControlOrMeta+k');
		const input = page.getByTestId('palette-input');
		for (const query of ['web', 'zzzz-no-match']) {
			await input.fill(query);
			await page.waitForTimeout(400);
			const { violations } = await new AxeBuilder({ page }).include('dialog').analyze();
			expect(violations.map((violation) => `${violation.id}: ${violation.nodes.map((node) => node.target.join(' ')).join(', ')}`), query).toEqual([]);
		}
		await expect(page.getByText(/Nothing matches|Aucun résultat/)).toBeVisible();
	});
});
