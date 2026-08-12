import { test, expect, type APIRequestContext } from '@playwright/test';

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

const templateContent = '#cloud-config\npackages:\n  - nginx\n';

test.describe('T18 admin cloud-init templates', () => {
	test.describe.configure({ mode: 'serial' });

	test('create a template, use it during simple-mode VM creation, then disable it (SC-001..SC-006)', async ({ page }) => {
		await signInAdmin(page.request);

		// SC-001: create a template as admin.
		await page.goto('/admin/cloudinit-templates');
		await page.getByRole('button', { name: 'New template' }).click();
		await page.getByLabel('Label').fill('Web server');
		await page.getByLabel('Content (must start with #cloud-config)').fill(templateContent);
		await page.getByRole('button', { name: 'Create' }).click();

		const templateRow = page.locator('tr', { hasText: 'Web server' });
		await expect(templateRow).toBeVisible();
		await expect(templateRow.getByRole('button', { name: 'Enabled' })).toBeVisible();

		// SC-002: the template appears in the simple-mode picker while enabled.
		await page.goto('/vms/create');
		await expect(page.getByLabel('Cloud-init template (optional)')).toBeVisible();
		const picker = page.getByLabel('Cloud-init template (optional)');
		await expect(picker.locator('option', { hasText: 'Web server' })).toHaveCount(1);

		// SC-003: select it during simple-mode VM creation and confirm the
		// resulting VM's cloud-init tab shows the template content.
		await page.getByLabel('Name').fill('cit-e2e-01');
		await page.getByRole('radio', { name: /small/ }).check();
		await picker.selectOption('web-server');
		await page.getByRole('button', { name: 'Create VM' }).click();

		// The VM list should now contain the new VM; navigate to its detail and
		// open the cloud-init tab to confirm the snippet content.
		await page.goto('/vms');
		const vmLink = page.getByRole('link', { name: /cit-e2e-01/ }).first();
		await expect(vmLink).toBeVisible();
		await vmLink.click();
		await page.getByRole('tab', { name: 'Cloud-init' }).click();
		await expect(page.locator('pre, textarea').filter({ hasText: 'nginx' })).toHaveCount(1);

		// SC-005/SC-006: disable the template and confirm it disappears from the
		// picker on a fresh create visit.
		await page.goto('/admin/cloudinit-templates');
		await templateRow.getByRole('button', { name: 'Enabled' }).click();
		await expect(templateRow.getByRole('button', { name: 'Disabled' })).toBeVisible();

		await page.goto('/vms/create');
		// The picker is only rendered when at least one enabled template exists;
		// with the sole template disabled, the field should be absent.
		await expect(page.getByLabel('Cloud-init template (optional)')).toHaveCount(0);
	});
});
