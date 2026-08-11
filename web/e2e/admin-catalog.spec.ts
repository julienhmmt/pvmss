import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

// Finds the row whose first (name) cell matches `name` exactly, further
// scoped to rows containing `context` — e.g. distinguishing "local" from
// "local-lvm", or "local"@pve-node-01 from "local"@pve-node-02.
function exactRow(page: Page, name: string, context?: string) {
	const row = page.locator('tr').filter({ has: page.locator('td', { hasText: new RegExp(`^${name}$`) }) });
	return context ? row.filter({ hasText: context }) : row;
}

test.describe('T11 admin catalog', () => {
	test.describe.configure({ mode: 'serial' });

	test('discovers and approves a node, storage, bridge, and ISO (US1, SC-001/002/007)', async ({ page }) => {
		await signInAdmin(page.request);

		// Nodes: pve-node-03 starts unapproved.
		await page.goto('/admin/nodes');
		const nodeRow = exactRow(page, 'pve-node-03');
		await expect(nodeRow.getByRole('button')).toHaveText('Approve');
		await nodeRow.getByRole('button').click();
		await expect(nodeRow.getByRole('button')).toHaveText('Approved');
		await page.reload();
		await expect(exactRow(page, 'pve-node-03').getByRole('button')).toHaveText('Approved');

		// Storages: local@pve-node-01 starts unapproved; local@pve-node-02 stays
		// approved and unaffected (FR-003 — per (name, node) pair, not by name).
		await page.goto('/admin/storages');
		const storageNode01 = exactRow(page, 'local', 'pve-node-01');
		const storageNode02 = exactRow(page, 'local', 'pve-node-02');
		await expect(storageNode01.getByRole('button')).toHaveText('Approve');
		await expect(storageNode02.getByRole('button')).toHaveText('Approved');
		await storageNode01.getByRole('button').click();
		await expect(storageNode01.getByRole('button')).toHaveText('Approved');
		await expect(storageNode02.getByRole('button')).toHaveText('Approved');

		// Bridges: vmbr2 starts unapproved.
		await page.goto('/admin/bridges');
		const bridgeRow = exactRow(page, 'vmbr2');
		await expect(bridgeRow.getByRole('button')).toHaveText('Approve');
		await bridgeRow.getByRole('button').click();
		await expect(bridgeRow.getByRole('button')).toHaveText('Approved');

		// ISOs: rocky-9 starts unapproved.
		await page.goto('/admin/isos');
		const isoRow = exactRow(page, 'rocky-9-generic-x86_64.iso');
		await expect(isoRow.getByRole('button')).toHaveText('Approve');
		await isoRow.getByRole('button').click();
		await expect(isoRow.getByRole('button')).toHaveText('Approved');

		// SC-002: the four approvals surface in T06's detailed create catalog
		// with zero changes to T06's own code.
		await page.goto('/vms/create');
		await page.getByRole('tab', { name: 'Detailed' }).click();
		await expect(page.getByLabel('Node').locator('option[value="pve-node-03"]')).toHaveCount(1);
		await expect(page.getByLabel('ISO').locator('option', { hasText: 'rocky-9-generic-x86_64.iso' })).toHaveCount(1);

		await page.getByLabel('Node').selectOption('pve-node-01');
		await page.getByRole('tab', { name: 'Disk' }).click();
		await expect(page.getByLabel('Storage').locator('option[value="local"]')).toHaveCount(1);

		await page.getByRole('tab', { name: 'Network' }).click();
		await expect(page.getByLabel('Bridge').locator('option[value="vmbr2"]')).toHaveCount(1);
	});

	test('creates, uses, disables, and re-enables a VM profile (US2, SC-005)', async ({ page }) => {
		await signInAdmin(page.request);

		await page.goto('/admin/profiles');
		await page.getByRole('button', { name: 'New profile' }).click();
		await page.getByLabel('Label').fill('XLarge');
		await page.getByLabel('vCPU cores').fill('8');
		await page.getByLabel('Memory (MB)').fill('16384');
		await page.getByLabel('Disk (GB)').fill('160');
		await page.getByRole('button', { name: 'Create' }).click();

		const profileRow = page.locator('tr', { hasText: 'XLarge' });
		await expect(profileRow).toBeVisible();
		await expect(profileRow.getByRole('button', { name: 'Enabled' })).toBeVisible();

		// Appears in T06's simple-mode picker while enabled.
		await page.goto('/vms/create');
		await expect(page.getByRole('radio', { name: /XLarge/ })).toBeVisible();

		// Disabling removes it from the picker but keeps it listed (disabled) here.
		await page.goto('/admin/profiles');
		await page.locator('tr', { hasText: 'XLarge' }).getByRole('button', { name: 'Enabled' }).click();
		await expect(page.locator('tr', { hasText: 'XLarge' }).getByRole('button', { name: 'Disabled' })).toBeVisible();

		await page.goto('/vms/create');
		await expect(page.getByRole('radio', { name: /XLarge/ })).toHaveCount(0);

		// Re-enabling restores it (FR-011: no cascade, nothing else to verify).
		await page.goto('/admin/profiles');
		await page.locator('tr', { hasText: 'XLarge' }).getByRole('button', { name: 'Disabled' }).click();
		await expect(page.locator('tr', { hasText: 'XLarge' }).getByRole('button', { name: 'Enabled' })).toBeVisible();
	});

	test('creates a tag, edits its color, and refuses to delete pvmss (US3, SC-006)', async ({ page }) => {
		await signInAdmin(page.request);

		await page.goto('/admin/tags');

		// pvmss is seeded, protected, and has no Delete control.
		const pvmssRow = page.locator('tr', { hasText: 'pvmss' });
		await expect(pvmssRow.getByText('protected')).toBeVisible();
		await expect(pvmssRow.getByRole('button', { name: 'Delete' })).toHaveCount(0);
		const deletePvmss = await page.request.delete('/api/v1/admin/tags/pvmss?cluster=default');
		expect(deletePvmss.status()).toBe(403);

		// Create a tag: VM count starts at 0.
		await page.getByRole('button', { name: 'New tag' }).click();
		await page.getByLabel(/Name/).fill('e2eteam');
		await page.getByRole('button', { name: 'Create' }).click();

		const tagRow = page.locator('tr', { hasText: 'e2eteam' });
		await expect(tagRow).toBeVisible();
		await expect(tagRow.locator('td').nth(2)).toHaveText('0');

		// Edit its color inline.
		await tagRow.getByRole('button', { name: 'Edit' }).click();
		await tagRow.locator('input[type="color"]').fill('#00ff00');
		await tagRow.getByRole('button', { name: 'Save' }).click();
		await expect(tagRow.getByText('#00ff00')).toBeVisible();

		// Delete it — succeeds, unlike pvmss.
		await tagRow.getByRole('button', { name: 'Delete' }).click();
		await expect(page.locator('tr', { hasText: 'e2eteam' })).toHaveCount(0);
	});

	test('a non-admin identity is redirected from the UI and rejected by the server (FR-008, SC-004)', async ({
		page
	}) => {
		const alice = await page.request.post('/api/v1/auth/login', {
			data: { username: 'alice', password: 'pvmss-alice' }
		});
		expect(alice.status()).toBe(200);

		await page.goto('/admin/nodes');
		await expect(page).toHaveURL(/\/login$/);

		const direct = await page.request.get('/api/v1/admin/nodes?cluster=default');
		expect(direct.status()).toBe(403);
	});
});
