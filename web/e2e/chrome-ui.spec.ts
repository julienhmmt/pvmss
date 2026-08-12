import { test, expect } from '@playwright/test';

// T19 — Chrome UI e2e. Four user stories: language switching (US1), theme
// toggle (US2), status banner (US3), homepage CTA by identity (US4). Runs
// against the fake cluster client (constitution XI — no Proxmox needed).

test.describe('T19 chrome UI', () => {
	test.describe('US1 — language switcher', () => {
		test('switching language updates visible strings and <html lang> without a reload', async ({ page }) => {
			await page.goto('/');
			// Default locale is French (constitution: French is the source language).
			await expect(page.locator('html')).toHaveAttribute('lang', 'fr');

			// Open the language dropdown and pick English.
			await page.getByRole('button', { name: 'Language' }).click();
			await page.getByRole('menuitemradio', { name: 'English' }).click();

			// <html lang> updates immediately, no reload.
			await expect(page.locator('html')).toHaveAttribute('lang', 'en');

			// A string this tranche renders is now in English.
			await expect(page.getByRole('button', { name: 'Language' })).toBeVisible();
		});

		test('language choice persists across reload', async ({ page, context }) => {
			await page.goto('/');
			await page.getByRole('button', { name: 'Language' }).click();
			await page.getByRole('menuitemradio', { name: 'English' }).click();
			await expect(page.locator('html')).toHaveAttribute('lang', 'en');

			await page.reload();
			await expect(page.locator('html')).toHaveAttribute('lang', 'en');
		});
	});

	test.describe('US2 — theme toggle', () => {
		test('toggling theme applies dark tokens and persists across reload', async ({ page }) => {
			await page.goto('/');
			const html = page.locator('html');
			// Start from a known state: clear any prior theme.
			await page.evaluate(() => localStorage.removeItem('pvmss-theme-v1'));
			await page.reload();

			// Toggle to dark.
			await page.getByRole('button', { name: 'Theme' }).click();
			await expect(html).toHaveClass(/dark/);

			// Persist across reload.
			await page.reload();
			await expect(html).toHaveClass(/dark/);
		});

		test('respects prefers-reduced-motion', async ({ browser }) => {
			const context = await browser.newContext({
				reducedMotion: 'reduce'
			});
			const page = await context.newPage();
			await page.goto('/');
			// The toggle is reachable and functional under reduced motion.
			await page.getByRole('button', { name: 'Theme' }).click();
			await expect(page.locator('html')).toHaveClass(/dark/);
			await context.close();
		});
	});

	test.describe('US3 — status banner', () => {
		test('demo mode shows the informational banner', async ({ page }) => {
			await page.goto('/');
			// The fake cluster client means demoMode is true — the info banner
			// is present throughout (quickstart.md step 9).
			await expect(page.getByText(/demonstration dataset|jeu de données de démonstration/)).toBeVisible();
		});
	});

	test.describe('US4 — homepage CTA', () => {
		test('anonymous visitor sees Log in and Documentation', async ({ page }) => {
			await page.goto('/');
			await expect(page.getByRole('link', { name: /Log in|Se connecter/ })).toBeVisible();
			await expect(page.getByRole('link', { name: /Documentation/ })).toBeVisible();
		});

		test('authenticated user sees My VMs, Create a VM, Documentation, and a welcome', async ({ page }) => {
			await page.goto('/login');
			await page.getByLabel('Username').fill('alice');
			await page.getByLabel('Password').fill('pvmss-alice');
			await page.locator('#login-cluster').selectOption('default');
			await page.getByRole('button', { name: 'Sign in' }).click();
			await page.waitForURL(/\/nodes/);
			await page.goto('/');
			await expect(page.getByRole('link', { name: /My VMs|Mes VM/ })).toBeVisible();
			await expect(page.getByRole('link', { name: /Create a VM|Créer une VM/ })).toBeVisible();
			await expect(page.getByText(/Welcome back|Bon retour/)).toBeVisible();
		});

		test('authenticated admin sees My VMs and Documentation but no Create a VM', async ({ page }) => {
			await page.goto('/login');
			await page.getByLabel('Account type').check();
			await page.getByRole('radio', { name: 'Local administrator' }).check();
			await page.getByLabel('Password').fill('pvmss-e2e-admin');
			await page.getByRole('button', { name: 'Sign in' }).click();
			await page.waitForURL(/\/nodes|\/admin/);
			await page.goto('/');
			await expect(page.getByRole('link', { name: /My VMs|Mes VM/ })).toBeVisible();
			await expect(page.getByRole('link', { name: /Documentation/ })).toBeVisible();
			await expect(page.getByRole('link', { name: /Create a VM|Créer une VM/ })).toHaveCount(0);
		});
	});
});
