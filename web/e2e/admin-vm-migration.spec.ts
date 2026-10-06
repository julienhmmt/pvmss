import { test, expect, type APIRequestContext, type Page } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

// monitor-01: running, pvmss-tagged, and not asserted on by any other spec.
const VMID = 112;
const VM_NAME = 'monitor-01';
const SOURCE = 'pve-node-01';
const TARGET = 'pve-node-02';
const SUCCESS_TIMEOUT_MS = 30_000;
const LOAD_TIMEOUT_MS = 15_000;

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

async function nodeOf(request: APIRequestContext, node: string): Promise<number[]> {
	const response = await request.get(`/api/v1/admin/nodes/default/${node}`);
	const body = (await response.json()) as { inventory: { vms: Array<{ vmid: number }> } };
	return body.inventory.vms.map((vm) => vm.vmid);
}

// Puts the VM back through the API so the shared fake fixture stays intact.
async function migrateBack(request: APIRequestContext): Promise<void> {
	if ((await nodeOf(request, TARGET)).includes(VMID)) {
		const response = await request.post(`/api/v1/admin/vms/default/${VMID}/migrate`, {
			data: { target: SOURCE },
			headers: await csrfHeaders(request)
		});
		expect(response.status()).toBe(202);
		const { upid } = (await response.json()) as { upid: string };
		await expect
			.poll(async () => ((await (await request.get(`/api/v1/tasks/${upid}?cluster=default`)).json()) as { state: string }).state, {
				timeout: SUCCESS_TIMEOUT_MS
			})
			.toBe('ok');
		await request.post('/api/v1/cluster/refresh', { headers: await csrfHeaders(request) });
	}
}

// Reaches the node page through the node list: a hard load of the node page
// does not fetch its details (afterNavigate does not fire on first load).
async function openNode(page: Page, node: string): Promise<void> {
	await page.goto('/admin/nodes');
	await page.getByRole('link', { name: node, exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`/admin/nodes/default/${node}$`));
}

function vmRow(page: Page) {
	return page.locator('tr', { hasText: VM_NAME });
}

test.describe('admin VM migration', () => {
	test.describe.configure({ mode: 'serial' });

	test.afterEach(async ({ page }) => {
		await migrateBack(page.request);
	});

	test('migrates a managed VM from the node page and follows it to the new node', async ({ page }) => {
		await signInAdmin(page.request);
		await openNode(page, SOURCE);

		await expect(vmRow(page)).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
		await page.getByTestId(`node-vm-migrate-${VMID}`).click();

		const dialog = page.getByRole('dialog');
		await expect(dialog.getByRole('heading', { name: new RegExp(`Migrate VM ${VMID}`) })).toBeVisible();
		await expect(dialog.getByText('Live migration')).toBeVisible();

		const targetOption = dialog.getByRole('radio', { name: TARGET });
		await expect(targetOption).toBeVisible();
		await expect(dialog.getByRole('radio', { name: SOURCE })).toHaveCount(0);

		await expect(dialog.getByTestId('migrate-continue')).toBeDisabled();
		await targetOption.check();
		await dialog.getByTestId('migrate-continue').click();

		await expect(dialog.getByTestId('migrate-summary')).toContainText(`from ${SOURCE} to ${TARGET}`);
		await dialog.getByTestId('migrate-confirm').click();

		await expect(dialog.getByTestId('migrate-result')).toContainText('migrated', { timeout: SUCCESS_TIMEOUT_MS });
		await dialog.getByRole('button', { name: 'Done' }).click();

		await expect(vmRow(page)).toHaveCount(0, { timeout: SUCCESS_TIMEOUT_MS });

		await openNode(page, TARGET);
		await expect(vmRow(page)).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
	});

	test('shows the preflight as keyboard-labelled options and can be closed with Escape', async ({ page }) => {
		await signInAdmin(page.request);
		await openNode(page, SOURCE);

		await page.getByRole('button', { name: `Migrate VM ${VMID} ${VM_NAME}` }).click();
		const dialog = page.getByRole('dialog');
		await expect(dialog.getByRole('group', { name: 'Target node' })).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(dialog).toHaveCount(0);
	});
});
