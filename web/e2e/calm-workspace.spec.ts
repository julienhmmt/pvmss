import { test, expect, type APIRequestContext, type Page } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

// Calm workspace migration (issue 12): the cross-cutting checks no single
// screen spec owns - the shell, the Activity and Account screens, the
// no-duplicate rule, responsive collapse and bilingual coverage.

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

async function useLocale(page: Page, locale: 'en' | 'fr'): Promise<void> {
	// Seed only when unset: switching language reloads the page, and the
	// init script must not undo the user's choice on that reload.
	await page.addInitScript((value) => {
		if (localStorage.getItem('pvmss-locale-seeded') === null) {
			localStorage.setItem('pvmss-locale', value);
			localStorage.setItem('pvmss-locale-seeded', '1');
		}
	}, locale);
}

test.describe('Calm workspace', () => {
	test.describe.configure({ mode: 'serial' });

	test('the sidebar offers exactly the three workspace destinations', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/create');
		const nav = page.getByTestId('app-sidebar').first().getByRole('navigation').first();
		await expect(nav.getByRole('link')).toHaveText([/My machines/, /Activity/, /Help & guides/]);
		// Create is a state of My machines, not an item of its own.
		await expect(nav.getByRole('link', { name: /My machines/ })).toHaveAttribute('aria-current', 'page');
		await expect(page.getByTestId('sidebar-reassurance').first()).toBeVisible();
		await expect(page.getByTestId('context-screen')).toHaveText('Create a machine');

		await page.getByTestId('sidebar-account-link').first().click();
		await expect(page).toHaveURL(/\/profile$/);
		await expect(page.getByTestId('account-name')).toHaveText('alice');
	});

	test('the account page switches theme and language, and both persist', async ({ page }) => {
		await useLocale(page, 'en');
		await signInAlice(page.request);
		await page.goto('/profile');

		const html = page.locator('html');
		const wasDark = (await html.getAttribute('class'))?.includes('dark') ?? false;
		await page.getByTestId('account-theme-toggle').click();
		await expect(html).toHaveClass(wasDark ? /^(?!.*\bdark\b)/ : /\bdark\b/);

		await page.getByTestId('account-language').selectOption('fr');
		await expect(html).toHaveAttribute('lang', 'fr');
		await expect(page.getByRole('heading', { name: 'Votre compte' })).toBeVisible();

		await page.reload();
		await expect(page.getByRole('heading', { name: 'Votre compte' })).toBeVisible();
		await expect(html).toHaveClass(wasDark ? /^(?!.*\bdark\b)/ : /\bdark\b/);
	});

	test('activity shows a power action in progress, then the recorded update', async ({ page }) => {
		await useLocale(page, 'en');
		await signInAlice(page.request);
		await page.goto('/activity');
		await expect(page.getByRole('heading', { name: 'Activity' })).toBeVisible();
		await expect(page.getByTestId('activity-recent')).toBeVisible();

		// Start a stopped machine from the list: while it converges the
		// sidebar counts it and the row reads "Starting".
		await page.goto('/vms?cluster=default');
		const row = page.getByTestId('vm-row').filter({ hasText: 'sandbox-02' });
		await row.getByTestId('vm-row-start').click();
		await expect(row).toHaveAttribute('data-status', /starting|running/);
		await expect(row).toHaveAttribute('data-status', 'running', { timeout: 15000 });

		// The audit trail now has the start, linked back to the machine.
		await page.getByTestId('app-sidebar').first().getByRole('link', { name: /Activity/ }).click();
		const update = page.getByTestId('activity-row').filter({ hasText: 'sandbox-02' }).first();
		await expect(update).toContainText('Started');
		await update.click();
		await expect(page).toHaveURL(/\/vms\/default\/115$/);

		// Leave the fixture as found: vm-list asserts on the stopped count.
		const stop = await page.request.post('/api/v1/vms/default/115/actions', {
			headers: await csrfHeaders(page.request),
			data: { action: 'stop' }
		});
		expect([200, 202, 409]).toContain(stop.status());
	});

	test('the create form refuses a name already used by one of my machines', async ({ page }) => {
		await useLocale(page, 'en');
		await signInAlice(page.request);
		await page.goto('/vms/create');
		await page.getByRole('button', { name: /Simple/ }).click();
		await page.getByRole('radio', { name: /Medium/ }).check();
		await page.getByLabel('Name').fill('web-01');
		await expect(page.getByText('You already have a machine with this name')).toBeVisible();
		await expect(page.getByTestId('vm-create-submit')).toBeDisabled();

		// The summary rail follows the form live.
		await page.getByLabel('Name').fill('web-e2e-fresh');
		await expect(page.getByTestId('summary-name')).toHaveText('web-e2e-fresh');
		await expect(page.getByTestId('vm-create-submit')).toBeEnabled();
	});

	test('the shell collapses: context header on desktop, menu bar on mobile', async ({ page }) => {
		await useLocale(page, 'en');
		await signInAlice(page.request);

		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/vms');
		await expect(page.getByTestId('context-header')).toBeVisible();
		await expect(page.getByTestId('sidebar-menu-button')).toBeHidden();

		await page.setViewportSize({ width: 390, height: 800 });
		await expect(page.getByTestId('context-header')).toBeHidden();
		await expect(page.getByTestId('sidebar-menu-button')).toBeVisible();
		// No horizontal page scroll at phone width.
		const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
		expect(overflow).toBeLessThanOrEqual(0);

		await page.getByTestId('sidebar-menu-button').click();
		await expect(page.locator('#app-sidebar-drawer').getByRole('link', { name: /My machines/ })).toBeVisible();
	});

	for (const locale of ['en', 'fr'] as const) {
		test(`every new screen renders without a raw message key (${locale})`, async ({ page }) => {
			await useLocale(page, locale);
			await signInAlice(page.request);
			for (const path of ['/vms', '/vms/default/100', '/vms/create', '/activity', '/docs', '/profile']) {
				await page.goto(path);
				await expect(page.locator('#page-heading, h1').first()).toBeVisible();
				const text = await page.locator('main').innerText();
				// A missing Paraglide key would surface as its dotted id.
				expect(text, `${path} (${locale})`).not.toMatch(/\b(vms|chrome|activity|account|help|machine)\.[a-z][\w.]+\b/);
			}
		});
	}
});
