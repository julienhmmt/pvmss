import { test, expect, type APIRequestContext, type Locator, type Page } from '@playwright/test';

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

/** The shared Pill's distinctive classes - the rendered status badge. */
function statusPill(row: Locator): Locator {
	return row.locator('span.inline-flex.whitespace-nowrap').first();
}

test.describe('Global search', () => {
	test('a machine shows the same status word on the list and in search', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		// web-01 is a running VM in the fake dataset.
		const listRow = page.locator('[data-testid="vm-row"]').filter({ hasText: 'web-01' });
		await expect(listRow).toBeVisible();
		const listWord = (await statusPill(listRow).textContent())?.trim();
		expect(listWord?.length).toBeGreaterThan(0);

		await page.goto('/search');
		await page.locator('[data-testid="global-search-input"]').fill('web-01');
		const resultRow = page.locator('[data-testid="search-result-row"]').filter({ hasText: 'web-01' });
		await expect(resultRow).toBeVisible();

		// One vocabulary: the search badge is the same pill with the same word.
		expect((await statusPill(resultRow).textContent())?.trim()).toBe(listWord);
	});
});
