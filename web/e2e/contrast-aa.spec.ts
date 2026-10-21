import { test, expect, type Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Ticket 31: text contrast stays at WCAG AA on the screens that failed the
// 2026-10-21 audit, in both themes (no stored theme, so the OS scheme wins).
const ADMIN_PASSWORD = 'pvmss-e2e-admin';

async function signInAlice(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	expect(response.status()).toBe(200);
}

async function signInAdmin(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/admin-login', { data: { password: ADMIN_PASSWORD } });
	expect(response.status()).toBe(200);
}

const ROUTES = [
	// The wizard is behind the mode chooser; its submit sits in the summary rail.
	{ path: '/vms/create', signIn: signInAlice, ready: '[data-testid="vm-create-submit"]', open: /Simple/ },
	{ path: '/vms/default/100', signIn: signInAlice, ready: 'main h1' },
	{ path: '/admin/policy/nodes', signIn: signInAdmin, ready: '[data-testid="node-details-link"]' }
] as const;

for (const colorScheme of ['light', 'dark'] as const) {
	for (const route of ROUTES) {
		test(`${route.path} meets AA text contrast in ${colorScheme}`, async ({ page }) => {
			await page.emulateMedia({ colorScheme });
			await route.signIn(page);
			await page.goto(route.path);
			if ('open' in route) await page.getByRole('button', { name: route.open }).click();
			await expect(page.locator(route.ready).first()).toBeVisible();

			const { violations } = await new AxeBuilder({ page }).withRules(['color-contrast']).analyze();
			const offenders = violations.flatMap((violation) => violation.nodes.map((node) => `${node.target.join(' ')}: ${node.any[0]?.message ?? ''}`));
			expect(offenders).toEqual([]);
		});
	}
}
