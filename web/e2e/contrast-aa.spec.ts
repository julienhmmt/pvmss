import { test, expect, type Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Ticket 31: text contrast stays at WCAG AA on the screens that failed the
// 2026-10-21 audit, in both themes (no stored theme, so the OS scheme wins).
// Named to run after admin-catalog.spec.ts, which approves the template whose
// "Recommended" badge failed; each route asserts its audited element is shown,
// so a pass is never a pass on a page without it.
const ADMIN_PASSWORD = 'pvmss-e2e-admin';

async function signInAlice(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	expect(response.status()).toBe(200);
}

async function signInAdmin(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/admin-login', { data: { password: ADMIN_PASSWORD } });
	expect(response.status()).toBe(200);
}

interface Route {
	path: string;
	signIn: (page: Page) => Promise<void>;
	/** The audited element; the check fails if it is not on screen. */
	ready: string;
	open?: (page: Page) => Promise<void>;
}

const ROUTES: Route[] = [
	// The wizard is behind the mode chooser.
	{ path: '/vms/create', signIn: signInAlice, ready: '[data-testid="source-card-template"]', open: (page) => page.getByRole('button', { name: /Simple/ }).click() },
	// VM 100 runs in the fake dataset, so its metrics stream shows "Live".
	{ path: '/vms/default/100', signIn: signInAlice, ready: '[data-testid="vm-metrics-stream-live"]', open: (page) => page.getByTestId('vm-tab-configuration').click() },
	{ path: '/admin/policy/nodes', signIn: signInAdmin, ready: '[data-testid="node-details-link"]' }
];

for (const colorScheme of ['light', 'dark'] as const) {
	for (const route of ROUTES) {
		test(`${route.path} meets AA text contrast in ${colorScheme}`, async ({ page }) => {
			await page.emulateMedia({ colorScheme });
			await route.signIn(page);
			await page.goto(route.path);
			await route.open?.(page);
			await expect(page.locator(route.ready).first()).toBeVisible({ timeout: 10_000 });

			const { violations } = await new AxeBuilder({ page }).withRules(['color-contrast']).analyze();
			const offenders = violations.flatMap((violation) => violation.nodes.map((node) => `${node.target.join(' ')}: ${node.any[0]?.message ?? ''}`));
			expect(offenders).toEqual([]);
		});
	}
}
