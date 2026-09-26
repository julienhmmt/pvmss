import { test, expect } from '@playwright/test';

// T19 - Chrome UI e2e. Four user stories: language switching (US1), theme
// toggle (US2), status banner (US3), homepage CTA by identity (US4). Runs
// against the fake cluster client (constitution XI - no Proxmox needed).
//
// The default locale is French (Paraglide base locale). SSR renders in French;
// the language switcher is a two-button group and the theme toggle is an icon
// button. Selectors use locale-agnostic patterns where possible.
//
// This file asserts the real French default, so it opts out of the suite-wide
// English seed that playwright.config.ts applies.
test.use({ storageState: { cookies: [], origins: [] } });

test.describe('T19 chrome UI', () => {
	test.describe('US1 - language switcher', () => {
		test('Layer B font swap: Archivo is applied and no Google Fonts request is made', async ({ page }) => {
			const googleFontRequests: string[] = [];
			page.on('request', (req) => {
				if (req.url().includes('fonts.googleapis.com')) googleFontRequests.push(req.url());
			});
			await page.goto('/');
			// The body font family must resolve to the self-hosted Archivo variable.
			const family = await page.evaluate(() => {
				const el = document.createElement('span');
				document.body.appendChild(el);
				const computed = getComputedStyle(el).fontFamily;
				el.remove();
				return computed;
			});
			expect(family.toLowerCase()).toContain('archivo');
			expect(googleFontRequests).toEqual([]);
		});

		test('switching language updates visible strings and <html lang> without a reload', async ({ page }) => {
			await page.goto('/');
			// Default locale is French (constitution: French is the source language).
			await expect(page.locator('html')).toHaveAttribute('lang', 'fr');

			// The language switcher exposes each locale as a button.
			await page.getByRole('button', { name: 'English' }).click();

			// <html lang> updates immediately, no reload.
			await expect(page.locator('html')).toHaveAttribute('lang', 'en');

			// The home CTA has switched to English.
			await expect(page.getByRole('link', { name: 'Log in' })).toBeVisible();
		});

		test('language choice persists across reload', async ({ page }) => {
			await page.goto('/');
			await page.getByRole('button', { name: 'English' }).click();
			await expect(page.locator('html')).toHaveAttribute('lang', 'en');

			await page.reload();
			await expect(page.locator('html')).toHaveAttribute('lang', 'en');
		});
	});

	test.describe('US2 - theme toggle', () => {
		test('toggling theme applies dark tokens and persists across reload', async ({ page }) => {
			await page.goto('/');
			const html = page.locator('html');
			// Start from a known state: clear any prior theme.
			await page.evaluate(() => localStorage.removeItem('pvmss-theme-v1'));
			await page.reload();

			// The theme toggle is an icon button labelled "Thème" / "Theme".
			await page.getByRole('button', { name: /Thème|Theme/ }).click();
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
			await page.getByRole('button', { name: /Thème|Theme/ }).click();
			await expect(page.locator('html')).toHaveClass(/dark/);
			await context.close();
		});
	});

	test.describe('US3 - status banner', () => {
		test('status banner is visible (demo mode or cluster degradation)', async ({ page }) => {
			await page.goto('/');
			// The fake cluster client has demoMode=true, but one cluster is
			// unreachable, so the "degraded" banner (which outranks "info")
			// is shown. Either way, a status banner must be visible.
			await expect(page.locator('[role="status"]')).toBeVisible();
		});
	});

	test.describe('US4 - homepage CTA', () => {
		// HomeCta renders <section class="flex flex-col items-center gap-4">.
		// The navbar's "My VMs" link is outside this section.
		const ctaSection = (page: import('@playwright/test').Page) =>
			page.locator('section.flex.flex-col.items-center.gap-4');

		test('anonymous visitor sees Log in and Documentation', async ({ page }) => {
			await page.goto('/');
			const cta = ctaSection(page);
			await expect(cta.getByRole('link', { name: /Se connecter|Log in/ })).toBeVisible();
			await expect(cta.getByRole('link', { name: /Documentation/ })).toBeVisible();
		});

		test('an authenticated pool user lands on their machines, not the pitch', async ({ page }) => {
			await page.goto('/login');
			await page.locator('input[autocomplete="username"]').fill('alice');
			await page.locator('input[type="password"]').fill('pvmss-alice');
			await page.locator('#login-cluster').selectOption('default');
			await page.locator('button[type="submit"]').click();
			// The machine list is the signed-in landing page (DESIGN.md §5).
			await page.waitForURL(/\/vms$/);
			await expect(page.getByRole('heading', { name: /Mes machines|My machines/ })).toBeVisible();
			await page.goto('/');
			await page.waitForURL(/\/vms$/);
		});

		test('authenticated admin is sent from the home page to the admin dashboard', async ({ page }) => {
			await page.goto('/login');
			await page.getByRole('button', { name: /administrat/i }).click();
			await page.locator('input[type="password"]').fill('pvmss-e2e-admin');
			await page.locator('button[type="submit"]').click();
			await page.waitForURL(/\/admin/);
			// The home page is the pool-user landing; an admin never sees its
			// My VMs / Create a VM calls to action and is redirected instead.
			await page.goto('/');
			await page.waitForURL(/\/admin$/);
			await expect(ctaSection(page)).toHaveCount(0);
		});
	});
});
