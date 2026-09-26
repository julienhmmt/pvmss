import { test, expect, type APIRequestContext } from '@playwright/test';
import { deleteVmsByPrefix } from './support/vms';

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

const templateContent = '#cloud-config\npackages:\n  - nginx\n';

test.describe('T18 admin cloud-init templates', () => {
	test.describe.configure({ mode: 'serial' });

	// This spec creates its VM through the UI; delete it afterwards so it does
	// not inflate the counts vm-list asserts on.
	test.afterAll(async ({ request }) => {
		await signInAlice(request);
		await deleteVmsByPrefix(request, 'cit-e2e-');
	});

	test('create a template, use it during simple-mode VM creation, then disable it (SC-001..SC-006)', async ({ page }) => {
		// Default locale is French; pin English so the English selectors match.
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		// SC-001: create a template as admin (only admins manage templates).
		await signInAdmin(page.request);
		await page.goto('/admin/cloudinit-templates');
		await page.getByTestId('cloudinit-new-template').click();
		await page.getByLabel('Label').fill('Web server');
		await page.getByLabel('Content (must start with #cloud-config)').fill(templateContent);
		await page.getByRole('button', { name: 'Create' }).click();

		const templateRow = page.locator('tr', { hasText: 'Web server' });
		await expect(templateRow).toBeVisible();
		await expect(templateRow.getByRole('switch', { name: 'Disable Web server' })).toBeVisible();

		// SC-002: the template appears in the simple-mode picker while enabled.
		// VM creation requires a pool-owning account (admins have no pool),
		// so alice performs the creation steps.
		await signInAlice(page.request);
		await page.goto('/vms/create');
		await page.getByRole('button', { name: /Simple/ }).click();
		const picker = page.getByLabel('Cloud-init document');
		await expect(picker).toBeVisible();
		await expect(picker.locator('option', { hasText: 'Web server' })).toHaveCount(1);

		// SC-003: select it during simple-mode VM creation and confirm the
		// resulting VM uses the template's published file.
		await page.getByLabel('Name').fill('cit-e2e-01');
		await page.getByRole('radio', { name: /small/i }).check();
		await picker.selectOption({ label: 'Web server' });
		await page.getByRole('button', { name: 'Create VM' }).click();
		// The creation is accepted asynchronously; wait for the success toast
		// so the VM is guaranteed to exist before we look for it. The task can
		// take longer than the default 5s assertion timeout.
		await expect(page.getByText('cit-e2e-01').first()).toBeVisible({ timeout: 20000 });
		await expect(page.getByText('created').first()).toBeVisible({ timeout: 20000 });

		// The VM list should now contain the new VM; filter by name so it
		// surfaces regardless of pagination, then open its detail.
		await page.goto('/vms');
		await page.getByTestId('vm-search').fill('cit-e2e-01');
		await expect(page).toHaveURL(/search=cit-e2e-01/);
		const vmLink = page.getByRole('link', { name: /cit-e2e-01/ }).first();
		await expect(vmLink).toBeVisible();
		await vmLink.click();
		await page.getByTestId('vm-tab-cloudinit').click();
		// The document mode names the published admin template the VM uses.
		await page.getByTestId('cloudinit-mode-document').click();
		await expect(page.getByTestId('cloudinit-document-current')).toContainText('Web server');
		const vmPath = new URL(page.url()).pathname.replace(/^\/vms\//, '');
		const before = await (await page.request.get(`/api/v1/vms/${vmPath}/cloudinit/document`)).json();

		// SC-004b: editing the source template publishes a NEW file; the
		// existing VM keeps the file it was created with.
		await signInAdmin(page.request);
		await page.goto('/admin/cloudinit-templates');
		await templateRow.getByRole('button', { name: 'Edit Web server' }).click();
		await page.getByLabel('Content (must start with #cloud-config)').fill('#cloud-config\npackages:\n  - postgresql\n');
		await page.getByRole('button', { name: 'Save' }).click();
		await expect(page.getByRole('dialog')).toHaveCount(0);

		await signInAlice(page.request);
		const after = await (await page.request.get(`/api/v1/vms/${vmPath}/cloudinit/document`)).json();
		expect(after.filename).toBe(before.filename);

		// SC-005/SC-006: disable the template and confirm it disappears from the
		// picker on a fresh create visit.
		await signInAdmin(page.request);
		await page.goto('/admin/cloudinit-templates');
		await templateRow.getByRole('switch', { name: 'Disable Web server' }).click();
		await expect(templateRow.getByRole('switch', { name: 'Enable Web server' })).toBeVisible();

		await signInAlice(page.request);
		await page.goto('/vms/create');
		await page.getByRole('button', { name: /Simple/ }).click();
		// The picker is only rendered when at least one enabled template exists;
		// with the sole template disabled, the field should be absent.
		await expect(page.getByLabel('Cloud-init document')).toHaveCount(0);
	});
});
