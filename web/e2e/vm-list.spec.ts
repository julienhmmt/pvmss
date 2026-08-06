import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

async function signIn(request: APIRequestContext, username: string, password: string): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username, password }
	});
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	await signIn(request, 'alice', 'pvmss-alice');
}

function vmRows(page: Page) {
	return page.locator('[data-testid="vm-row"]');
}

async function rowNames(page: Page): Promise<string[]> {
	const names = await vmRows(page).locator('td:nth-child(2)').allTextContents();
	return names.map((name) => name.trim());
}

test.describe('T04 VM list', () => {
	test('lists exactly the signed-in user\'s VMs with a quota counter', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms');

		// alice owns 7 VMs in the fake dataset (pool-alice: 100, 101, 102, 114, 115, 123, 124).
		await expect(vmRows(page)).toHaveCount(7);
		await expect(page.getByText('web-01')).toBeVisible();
		await expect(page.locator('[data-testid="vm-quota"]')).toContainText('7 VMs · unlimited');
	});

	test('one search field finds VMs by name, tag, or ID', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms');

		const search = page.locator('[data-testid="vm-search"]');

		// By name substring.
		await search.fill('web');
		await expect(vmRows(page)).toHaveCount(2);
		await expect(page).toHaveURL(/[?&]search=web/);

		// By tag — "db" also name-matches the sandbox VMs; the union is 3 rows.
		await search.fill('db');
		await expect(vmRows(page)).toHaveCount(3);
		await expect(page.getByText('db-01')).toBeVisible();

		// By numeric ID.
		await search.fill('114');
		await expect(vmRows(page)).toHaveCount(1);
		await expect(page.getByText('sandbox-01')).toBeVisible();

		// No match → distinct "no match" state, not the "no VMs at all" one.
		await search.fill('does-not-exist');
		await expect(page.locator('[data-testid="vm-empty-match"]')).toBeVisible();
	});

	test('status filter combines with an active search', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms');

		await page.locator('[data-testid="vm-search"]').fill('web');
		await expect(vmRows(page)).toHaveCount(2);

		await page.locator('[data-testid="vm-status-filter"]').selectOption('stopped');
		await expect(vmRows(page)).toHaveCount(1);
		await expect(page.getByText('web-02')).toBeVisible();
		await expect(page).toHaveURL(/[?&]search=web/);
		await expect(page).toHaveURL(/[?&]status=stopped/);
	});

	test('node filter narrows results without shrinking its own dropdown', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms');

		const nodeFilter = page.locator('[data-testid="vm-node-filter"]');
		await nodeFilter.selectOption('pve-node-02');
		await expect(vmRows(page)).toHaveCount(2);

		// Facet is computed pre-filter: both nodes stay selectable.
		await expect(nodeFilter.locator('option', { hasText: 'pve-node-01' })).toHaveCount(1);
		await expect(nodeFilter.locator('option', { hasText: 'pve-node-02' })).toHaveCount(1);
	});

	test('column headers sort, and clicking again reverses direction', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms');

		const idHeader = page.locator('[data-testid="sort-vmid"]');
		// Auto-retrying text assertions: the row order updates only after the
		// sorted fetch resolves, so never assert immediately after the click.
		const firstRowName = vmRows(page).first().locator('td:nth-child(2)');
		await expect(vmRows(page)).toHaveCount(7);

		// Default sort is name ascending: db-01 first.
		await expect(firstRowName).toHaveText('db-01');

		await idHeader.click();
		await expect(page).toHaveURL(/[?&]sortBy=vmid/);
		await expect(firstRowName).toHaveText('web-01');
		await expect(page.locator('th', { has: idHeader })).toHaveAttribute('aria-sort', 'ascending');

		await idHeader.click();
		await expect(page).toHaveURL(/[?&]sortDir=desc/);
		await expect(firstRowName).toHaveText('dev-02');
		await expect(page.locator('th', { has: idHeader })).toHaveAttribute('aria-sort', 'descending');
	});

	test('pagination moves forward and back through pages', async ({ page }) => {
		await signInAlice(page.request);
		// 7 VMs with pageSize=2 → 4 pages.
		await page.goto('/vms?pageSize=2');
		await expect(vmRows(page)).toHaveCount(2);
		await expect(page.locator('[data-testid="vm-page-indicator"]')).toContainText('Page 1 of 4');

		await page.locator('[data-testid="vm-page-next"]').click();
		await expect(page.locator('[data-testid="vm-page-indicator"]')).toContainText('Page 2 of 4');
		await expect(page).toHaveURL(/[?&]page=2/);
		const pageTwoNames = await rowNames(page);

		await page.locator('[data-testid="vm-page-prev"]').click();
		await expect(page.locator('[data-testid="vm-page-indicator"]')).toContainText('Page 1 of 4');
		const pageOneNames = await rowNames(page);

		expect(pageOneNames).not.toEqual(pageTwoNames);
	});

	test('a reloaded URL reproduces the exact same state (SC-004)', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms');

		await page.locator('[data-testid="vm-search"]').fill('pvmss');
		await page.locator('[data-testid="vm-status-filter"]').selectOption('stopped');
		await page.locator('[data-testid="sort-vmid"]').click();
		await expect(vmRows(page)).toHaveCount(4);

		const url = page.url();
		const beforeNames = await rowNames(page);

		await page.goto(url);
		await expect(vmRows(page)).toHaveCount(4);
		expect(await rowNames(page)).toEqual(beforeNames);
		await expect(page.locator('[data-testid="vm-search"]')).toHaveValue('pvmss');
		await expect(page.locator('[data-testid="vm-status-filter"]')).toHaveValue('stopped');
	});

	test('bob sees a completely different set of VMs, never alice\'s', async ({ page }) => {
		await signIn(page.request, 'bob', 'pvmss-bob');
		await page.goto('/vms');

		// bob owns pool-bob (103, 104, 105, 106, 116, 117, 118) — 7 VMs, none alice's.
		await expect(vmRows(page)).toHaveCount(7);
		expect(await rowNames(page)).not.toContain('web-01');
		await expect(page.getByText('cache-01')).toBeVisible();
	});

	test('an unmatched search shows the no-match state, distinct from no-VMs-owned', async ({ page }) => {
		// T008 covers no_vms_owned at HTTP level; here the two empty states must
		// not collapse into one another (FR-008).
		await signInAlice(page.request);
		await page.goto('/vms?search=no-such-vm');
		await expect(page.locator('[data-testid="vm-empty-match"]')).toBeVisible();
		await expect(page.locator('[data-testid="vm-empty-owned"]')).toBeHidden();
	});
});
