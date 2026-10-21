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

		// At least one seeded user-facing page should be listed as an article.
		await expect(page.getByTestId('help-article').filter({ hasText: /Getting started|Premiers pas|User guide|Guide de l'utilisateur/i }).first()).toBeVisible();
		// The first article is open and its body is rendered in place.
		await expect(page.getByTestId('help-article').first()).toHaveAttribute('open', '');
		await expect(page.getByTestId('help-article').first().locator('article')).toBeVisible();
		await expect(page.getByTestId('help-aside')).toBeVisible();
		// The page heading is the only h1; article bodies start at h2.
		await expect(page.locator('h1')).toHaveCount(1);
		// Help boxes are page content, not complementary landmarks nested in main.
		await expect(page.locator('main aside')).toHaveCount(0);
	});

	test('docs list is reachable while signed out', async ({ page }) => {
		await page.goto('/docs');
		await expect(page.getByRole('heading', { name: /You don't need to know Proxmox|Pas besoin de connaître Proxmox/ })).toBeVisible();
		// No redirect to a sign-in wall.
		await expect(page.getByText('Authentification requise')).toHaveCount(0);
	});

	test('a documentation page renders its HTML body', async ({ page }) => {
		await page.goto('/docs/user-guide');
		await expect(page.getByText('Chargement…')).toHaveCount(0, { timeout: 10_000 });
		// The rendered body (markdown -> HTML) should be present.
		await expect(page.locator('article')).toBeVisible();
	});

	test('documentation link preserves the selected language in the detail page', async ({ page }) => {
		await page.goto('/docs');
		await expect(page.getByText('Chargement…')).toHaveCount(0, { timeout: 10_000 });

		// Switch to English and open a user-facing page.
		await page.getByRole('button', { name: 'English' }).click();
		const article = page.getByTestId('help-article').filter({ has: page.getByText('User guide', { exact: true }) });
		await article.locator('summary').click();
		await article.getByTestId('help-open-page').click();

		await expect(page).toHaveURL(/\/docs\/user-guide\?lang=en$/);
		// The body does not repeat the title: one heading, the page h1.
		await expect(page.getByRole('heading', { name: 'User guide' })).toHaveCount(1);
		await expect(page.locator('h1')).toHaveCount(1);
	});

	test('language selector filters the list by language', async ({ page }) => {
		await page.goto('/docs');
		await expect(page.getByText('Chargement…')).toHaveCount(0, { timeout: 10_000 });

		// Selecting English shows only the English user-facing pages.
		await page.getByRole('button', { name: 'English' }).click();
		await expect(page.getByTestId('help-article').filter({ hasText: /Getting started|User guide|VM creation guidelines/ })).toHaveCount(3);
		await expect(page.getByText("Guide de l'utilisateur")).toHaveCount(0);

		// Switching to French shows only the French variants.
		await page.getByRole('button', { name: 'Français' }).click();
		await expect(page.getByText("Guide de l'utilisateur")).toBeVisible();
		await expect(page.getByText('Getting started')).toHaveCount(0);
	});
});
