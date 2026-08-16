import { test, expect } from '@playwright/test';

// Regression guard for issue #53 public documentation browser. The docs list
// must render without the user needing to be authenticated, and the loading
// spinner must clear once /api/v1/docs resolves (a class-store reactivity bug
// previously left it stuck on "Chargement…").
test.describe('T53 documentation browser', () => {
	test('public docs list renders without a stuck loading state', async ({ page }) => {
		await page.goto('/docs');

		// The spinner must clear: wait for either the list or an error, and
		// assert the loading text is gone.
		await expect(page.getByText('Chargement…')).toHaveCount(0, { timeout: 10_000 });

		// At least one seeded user-facing page should be visible.
		await expect(page.getByRole('link', { name: /Getting started|Guide de l'utilisateur|User guide/i })).toBeVisible();
	});

	test('docs list is reachable while signed out', async ({ page }) => {
		await page.goto('/docs');
		await expect(page.getByRole('heading', { name: 'Documentation' })).toBeVisible();
		// No redirect to a sign-in wall.
		await expect(page.getByText('Authentification requise')).toHaveCount(0);
	});

	test('a documentation page renders its HTML body', async ({ page }) => {
		await page.goto('/docs/user-guide');
		await expect(page.getByText('Chargement…')).toHaveCount(0, { timeout: 10_000 });
		// The rendered body (markdown -> HTML) should be present.
		await expect(page.locator('article')).toBeVisible();
	});
});
