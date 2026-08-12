import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

async function signIn(request: APIRequestContext, username: string, password: string): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username, password, cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	await signIn(request, 'alice', 'pvmss-alice');
}

function vmRowCheckboxes(page: Page) {
	return page.locator('[data-testid="vm-bulk-select-row"]');
}

function selectAllCheckbox(page: Page) {
	return page.locator('[data-testid="vm-bulk-select-all"]');
}

test.describe('T17 VM bulk actions', () => {
	test('select several VMs and bulk-start them, showing a per-VM result', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		// Wait for the list to load — alice owns 7 VMs.
		await expect(page.locator('[data-testid="vm-row"]')).toHaveCount(7);

		// Select two stopped VMs (101 = web-02 stopped, 124 = dev-02 stopped).
		const checkboxes = vmRowCheckboxes(page);
		await checkboxes.nth(1).check();
		await checkboxes.nth(6).check();

		// The bulk action bar appears with the selected count.
		await expect(page.locator('[data-testid="vm-bulk-action-bar"]')).toBeVisible();
		await expect(page.locator('[data-testid="vm-bulk-selected-count"]')).toContainText('2 selected');

		// Choose "start" and apply.
		await page.locator('[data-testid="vm-bulk-action-select"]').selectOption('start');
		await page.locator('[data-testid="vm-bulk-action-submit"]').click();

		// The per-VM result panel appears with one row per target.
		await expect(page.locator('[data-testid="vm-bulk-result-panel"]')).toBeVisible();
		await expect(page.locator('[data-testid="vm-bulk-result-row"]')).toHaveCount(2);
		await expect(page.locator('[data-testid="vm-bulk-result-ok"]')).toHaveCount(2);

		// The summary shows 2 succeeded, 0 failed.
		await expect(page.locator('[data-testid="vm-bulk-result-summary"]')).toContainText('2');
	});

	test('select-all checkbox selects every VM on the current page', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default&pageSize=10');

		await expect(page.locator('[data-testid="vm-row"]')).toHaveCount(7);

		await selectAllCheckbox(page).check();
		await expect(page.locator('[data-testid="vm-bulk-selected-count"]')).toContainText('7 selected');

		// Unchecking select-all clears the page selection.
		await selectAllCheckbox(page).uncheck();
		await expect(page.locator('[data-testid="vm-bulk-action-bar"]')).not.toBeVisible();
	});

	test('mixed success and failure shows per-target outcomes, not one aggregate banner', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		await expect(page.locator('[data-testid="vm-row"]')).toHaveCount(7);

		// Select a stopped VM (101 = web-02) and a running VM (100 = web-01).
		// Starting the running one will fail with "vm already running" (T001b).
		const checkboxes = vmRowCheckboxes(page);
		await checkboxes.nth(0).check(); // web-01 (running)
		await checkboxes.nth(1).check(); // web-02 (stopped)

		await page.locator('[data-testid="vm-bulk-action-select"]').selectOption('start');
		await page.locator('[data-testid="vm-bulk-action-submit"]').click();

		// The result panel shows 2 rows — one ok, one error.
		await expect(page.locator('[data-testid="vm-bulk-result-panel"]')).toBeVisible();
		await expect(page.locator('[data-testid="vm-bulk-result-row"]')).toHaveCount(2);
		await expect(page.locator('[data-testid="vm-bulk-result-ok"]')).toHaveCount(1);
		await expect(page.locator('[data-testid="vm-bulk-result-error"]')).toHaveCount(1);

		// The summary shows mixed counts.
		await expect(page.locator('[data-testid="vm-bulk-result-summary"]')).toContainText('1');
	});

	test('clear selection button empties the selection and hides the bar', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		await expect(page.locator('[data-testid="vm-row"]')).toHaveCount(7);

		await vmRowCheckboxes(page).nth(0).check();
		await expect(page.locator('[data-testid="vm-bulk-action-bar"]')).toBeVisible();

		await page.locator('[data-testid="vm-bulk-clear-selection"]').click();
		await expect(page.locator('[data-testid="vm-bulk-action-bar"]')).not.toBeVisible();
	});

	test('dismiss result button hides the result panel', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		await expect(page.locator('[data-testid="vm-row"]')).toHaveCount(7);

		await vmRowCheckboxes(page).nth(1).check();
		await page.locator('[data-testid="vm-bulk-action-select"]').selectOption('start');
		await page.locator('[data-testid="vm-bulk-action-submit"]').click();

		await expect(page.locator('[data-testid="vm-bulk-result-panel"]')).toBeVisible();
		await page.locator('[data-testid="vm-bulk-dismiss-result"]').click();
		await expect(page.locator('[data-testid="vm-bulk-result-panel"]')).not.toBeVisible();
	});
});
