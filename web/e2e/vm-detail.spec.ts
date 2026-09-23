import { test, expect, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './support/csrf';
import { cleanupCreatedVms, createVm } from './support/vms';

async function signIn(request: APIRequestContext, username: string, password: string): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username, password, cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	await signIn(request, 'alice', 'pvmss-alice');
}

test.describe('T05 VM detail & actions (closes S01)', () => {
	// The specs below mutate VMs (start, rename, delete), so they act on VMs
	// they create. The fake dataset is shared by the whole run, so touching a
	// fixture would break every later spec that asserts the pristine set.
	test.afterEach(async ({ request }) => {
		await signInAlice(request);
		await cleanupCreatedVms(request);
	});
	test('opens a VM from the list and sees identity, status, and metrics', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		// Click the web-01 row link (VMID 100, running). Default list sort is
		// by name, so "first row" isn't deterministically web-01 - target it explicitly.
		await page.getByTestId('vm-row-link').filter({ hasText: 'web-01' }).click();
		await expect(page).toHaveURL(/\/vms\/default\/100$/);

		await expect(page.getByTestId('vm-name')).toHaveText('web-01');
		await expect(page.getByTestId('vm-status')).toContainText('running');
		await expect(page.getByTestId('vm-meta')).toContainText('pve-node-01');
		// Locale-agnostic: default locale is French ("2 cœurs"), not English.
		await expect(page.getByTestId('vm-stat-cpu')).toContainText('2');
		await expect(page.getByTestId('vm-stat-uptime')).toBeVisible();
	});

	test('T02: metrics history row renders and the range toggle switches without error', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100');

		await expect(page.getByTestId('vm-metrics-row')).toBeVisible();
		await expect(page.getByTestId('vm-metrics-charts')).toBeVisible({ timeout: 10000 });
		await expect(page.getByTestId('line-chart')).toHaveCount(4);

		// Default range is "hour" - pressed state reflects it.
		await expect(page.getByTestId('vm-metrics-range-hour')).toHaveAttribute('aria-pressed', 'true');

		await page.getByTestId('vm-metrics-range-day').click();
		await expect(page.getByTestId('vm-metrics-range-day')).toHaveAttribute('aria-pressed', 'true');
		await expect(page.getByTestId('vm-metrics-charts')).toBeVisible();
		await expect(page.getByTestId('vm-metrics-error')).toBeHidden();

		await page.getByTestId('vm-metrics-range-week').click();
		await expect(page.getByTestId('vm-metrics-range-week')).toHaveAttribute('aria-pressed', 'true');
		await expect(page.getByTestId('vm-metrics-charts')).toBeVisible();
		await expect(page.getByTestId('vm-metrics-error')).toBeHidden();
	});

	test('start action on a stopped VM flips status optimistically then reconciles', async ({ page }) => {
		await signInAlice(page.request);
		// A VM of this test's own, created stopped - starting a fixture would
		// change the stopped counts that vm-list asserts on.
		const vm = await createVm(page.request, 'detail-e2e-start');
		await page.goto(`/vms/default/${vm.vmid}`);

		await expect(page.getByTestId('vm-status')).toContainText('stopped');
		await page.getByTestId('vm-action-start').click();

		// After the action, the status reconciles to running.
		await expect(page.getByTestId('vm-status')).toContainText('running');
	});

	test('delete opens a confirmation dialog, confirms, and the VM disappears', async ({ page }) => {
		await signInAlice(page.request);
		const vm = await createVm(page.request, 'detail-e2e-del');
		await page.goto(`/vms/default/${vm.vmid}`);

		await page.getByTestId('vm-action-delete').click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await page.getByTestId('vm-delete-confirm').click();

		// After delete, navigates back to the list.
		await expect(page).toHaveURL(/\/vms$/);
	});

	test('delete a running VM prompts for force-stop, then confirms and deletes', async ({ page }) => {
		await signInAlice(page.request);
		const vm = await createVm(page.request, 'detail-e2e-run', { start: true });
		await page.goto(`/vms/default/${vm.vmid}`);

		await page.getByTestId('vm-action-delete').click();
		await expect(page.getByRole('dialog')).toBeVisible();

		// First confirm - the server reports the VM is running.
		await page.getByTestId('vm-delete-confirm').click();

		// The dialog switches to the force-stop warning step.
		await expect(page.getByTestId('vm-delete-running-warning')).toBeVisible();

		// Second confirm - force-stops and deletes.
		await page.getByTestId('vm-delete-confirm').click();

		// After delete, navigates back to the list.
		await expect(page).toHaveURL(/\/vms$/);
	});

	test('S01 closure: a non-owner cannot stop a VM they do not own (SC-001)', async ({ request }) => {
		await signIn(request, 'alice', 'pvmss-alice');
		const vm = await createVm(request, 'detail-e2e-owner', { start: true });

		// This is S01's exact PoC request, now expected to fail.
		await signIn(request, 'bob', 'pvmss-bob');
		const response = await request.post(`/api/v1/vms/default/${vm.vmid}/actions`, {
			headers: await csrfHeaders(request),
			data: { action: 'stop' }
		});
		expect(response.status()).toBe(403);
		const body = await response.json();
		expect(body.code).toBe('forbidden');
	});

	test('S01 closure: a forged node field is rejected at decode time', async ({ request }) => {
		await signInAlice(request);
		const response = await request.post('/api/v1/vms/default/100/actions', {
			headers: await csrfHeaders(request),
			data: { action: 'start', node: 'pve-node-evil' }
		});
		expect(response.status()).toBe(400);
	});

	test('rename inline: type a new name, press Enter, it persists', async ({ page }) => {
		await signInAlice(page.request);
		const vm = await createVm(page.request, 'detail-e2e-ren');
		await page.goto(`/vms/default/${vm.vmid}`);

		await page.getByTestId('vm-name').click();
		await page.getByTestId('vm-name-edit').fill('detail-e2e-renamed');
		await page.keyboard.press('Enter');

		await expect(page.getByTestId('vm-name')).toHaveText('detail-e2e-renamed');
	});

	test('T03: live metrics tick updates the running VM chart', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100');

		await expect(page.getByTestId('vm-metrics-charts')).toBeVisible({ timeout: 10000 });
		await expect(page.getByTestId('line-chart')).toHaveCount(4);

		const cpuChart = page.getByTestId('line-chart').first();
		const cpuPath = cpuChart.locator('path');
		const initialPath = await cpuPath.getAttribute('d');

		// The running VM opens an SSE stream; within a few seconds a live tick
		// arrives and the SVG path changes.
		await expect
			.poll(async () => cpuChart.getAttribute('d'), {
				timeout: 10000,
				message: 'expected the CPU chart to receive a live tick'
			})
			.not.toBe(initialPath);
	});

	test('a non-owner opening a VM by URL gets 403/404, no data leaks', async ({ page }) => {
		// Bob opens alice's VM 100 by editing the URL. Sign in via page.request
		// (not the standalone `request` fixture) so the session cookie lands in
		// the browser context the page itself uses - a separate APIRequestContext
		// has its own cookie jar and never reaches the page.
		await signIn(page.request, 'bob', 'pvmss-bob');
		await page.goto('/vms/default/100');
		// The detail page shows an error, never the VM's data.
		await expect(page.getByTestId('vm-detail-error')).toBeVisible();
		await expect(page.getByText('web-01')).toHaveCount(0);
	});
});
