import { test, expect, type Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Full WCAG 2.1 A/AA axe scan of the main user and admin screens, in both
// themes. contrast-aa.spec.ts pins specific audited elements; this one keeps
// every rule on so a new unlabelled control or broken ARIA fails the suite.
const ADMIN_PASSWORD = 'pvmss-e2e-admin';

async function signInAlice(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	expect(response.status()).toBe(200);
}

async function signInAdmin(page: Page): Promise<void> {
	const response = await page.request.post('/api/v1/auth/admin-login', { data: { password: ADMIN_PASSWORD } });
	expect(response.status()).toBe(200);
}

const USER_ROUTES = ['/vms', '/vms/default/100', '/vms/create'];
const ADMIN_ROUTES = ['/admin', '/admin/vms', '/admin/nodes', '/admin/templates', '/admin/profiles', '/admin/tags', '/admin/policy', '/admin/clusters'];

const ROUTES = [
	...USER_ROUTES.map((path) => ({ path, signIn: signInAlice })),
	...ADMIN_ROUTES.map((path) => ({ path, signIn: signInAdmin }))
];

for (const colorScheme of ['light', 'dark'] as const) {
	for (const route of ROUTES) {
		test(`${route.path} has no WCAG A/AA violations in ${colorScheme}`, async ({ page }) => {
			await page.emulateMedia({ colorScheme });
			await route.signIn(page);
			await page.goto(route.path);
			await expect(page.locator('main h1')).toBeVisible({ timeout: 10_000 });
			await page.waitForLoadState('networkidle');

			const { violations } = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
			const offenders = violations.flatMap((v) => v.nodes.map((node) => `[${v.id}] ${node.target.join(' ')}: ${node.failureSummary ?? v.help}`));
			expect(offenders).toEqual([]);
		});
	}
}
