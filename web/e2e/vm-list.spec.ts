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

function vmRows(page: Page) {
	return page.locator('[data-testid="vm-row"]');
}

async function rowNames(page: Page): Promise<string[]> {
	const names = await vmRows(page).locator('[data-testid="vm-row-link"]').allTextContents();
	return names.map((name) => name.trim());
}

test.describe('T04 VM list', () => {
	test('lists exactly the signed-in user\'s VMs with a quota counter', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		// alice owns 7 VMs in the fake dataset (pool-alice: 100, 101, 102, 114, 115, 123, 124).
		await expect(vmRows(page)).toHaveCount(7);
		await expect(page.getByText('web-01')).toBeVisible();
		await expect(page.locator('[data-testid="vm-quota"]')).toContainText('7 machines, no limit set');
		await expect(page.locator('[data-testid="vm-count"]')).toHaveText('7 machines');
	});

	test('one search field finds VMs by name, tag, or ID', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		const search = page.locator('[data-testid="vm-search"]');

		// By name substring.
		await search.fill('web');
		await expect(vmRows(page)).toHaveCount(2);
		await expect(page).toHaveURL(/[?&]search=web/);

		// By tag - "db" also name-matches the sandbox VMs; the union is 3 rows.
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
		await page.goto('/vms?cluster=default');

		await page.locator('[data-testid="vm-search"]').fill('web');
		await expect(vmRows(page)).toHaveCount(2);

		await page.locator('[data-testid="vm-status-filter"]').selectOption('stopped');
		await expect(vmRows(page)).toHaveCount(1);
		await expect(page.getByText('web-02')).toBeVisible();
		await expect(page).toHaveURL(/[?&]search=web/);
		await expect(page).toHaveURL(/[?&]status=stopped/);
	});

	test('a stopped machine explains itself and offers Start; running ones never claim Connect', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		const stopped = vmRows(page).filter({ hasText: 'dev-02' });
		await expect(stopped).toHaveAttribute('data-status', 'stopped');
		await expect(stopped.getByTestId('vm-row-hint')).toContainText('Your files are kept');
		await expect(stopped.getByTestId('vm-row-start')).toBeVisible();

		const running = vmRows(page).filter({ hasText: 'web-01' });
		await expect(running).toHaveAttribute('data-status', 'running');
		await expect(running.getByTestId('vm-row-details')).toBeVisible();
		await expect(running.getByTestId('vm-row-hint')).toHaveCount(0);
		await expect(page.getByRole('link', { name: /^Connect/ })).toHaveCount(0);
	});

	test('rows are ordered by name', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');
		await expect(vmRows(page)).toHaveCount(7);
		const names = await rowNames(page);
		expect(names).toEqual([...names].sort());
		expect(names[0]).toBe('db-01');
	});

	test('pagination moves forward and back through pages', async ({ page }) => {
		await signInAlice(page.request);
		// 7 VMs with pageSize=2 → 4 pages.
		await page.goto('/vms?cluster=default&pageSize=2');
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
		await page.goto('/vms?cluster=default');

		await page.locator('[data-testid="vm-search"]').fill('pvmss');
		await page.locator('[data-testid="vm-status-filter"]').selectOption('stopped');
		await expect(page).toHaveURL(/[?&]status=stopped/);
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
		await page.goto('/vms?cluster=default');

		// bob owns pool-bob (103, 104, 105, 106, 116, 117, 118) - 7 VMs, none alice's.
		await expect(vmRows(page)).toHaveCount(7);
		expect(await rowNames(page)).not.toContain('web-01');
		await expect(page.getByText('cache-01')).toBeVisible();
	});

	test('an unmatched search shows the no-match state, distinct from no-VMs-owned', async ({ page }) => {
		// T008 covers no_vms_owned at HTTP level; here the two empty states must
		// not collapse into one another (FR-008).
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default&search=no-such-vm');
		await expect(page.locator('[data-testid="vm-empty-match"]')).toBeVisible();
		await expect(page.locator('[data-testid="vm-empty-owned"]')).toBeHidden();

		// Clear filters brings the whole collection back.
		await page.getByTestId('vm-clear-filters').click();
		await expect(vmRows(page)).toHaveCount(7);
		await expect(page.locator('[data-testid="vm-search"]')).toHaveValue('');
	});

	test('the whole row opens the VM detail, not just the name link', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		// The name link is stretched over the whole row: a click on the
		// resources line must still navigate. Use a real pointer click.
		const resources = vmRows(page).first().getByText(/vCPU/);
		const box = await resources.boundingBox();
		if (box === null) throw new Error('resources line has no box');
		await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);

		await expect(page).toHaveURL(/\/vms\/default\/\d+$/);
	});

	test('the console is one click away from a running machine, in a new tab', async ({ page, context }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		await vmRows(page).filter({ hasText: 'web-01' }).getByTestId('vm-row-details').click();
		await expect(page).toHaveURL(/\/vms\/default\/100$/);

		const consoleLink = page.getByTestId('vm-console-open');
		await expect(consoleLink).toBeVisible();
		const [popup] = await Promise.all([context.waitForEvent('page'), consoleLink.click()]);
		await popup.waitForURL('/vms/default/*/console');
	});
});
