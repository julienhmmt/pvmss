import { test, expect, type APIRequestContext } from '@playwright/test';

async function signIn(request: APIRequestContext, username: string, password: string): Promise<void> {
	const response = await request.post('/api/v1/auth/login', { data: { username, password, cluster: 'default' } });
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	await signIn(request, 'alice', 'pvmss-alice');
}

test.describe.configure({ mode: 'serial' });

test.describe('T08 VM cloud-init', () => {
	test('edits structured config with explicit confirmation and reloads', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/102');
		await page.getByTestId('vm-tab-configuration').click();
		await page.getByTestId('vm-tab-cloudinit').click();
		await expect(page.getByTestId('cloudinit-user')).toHaveValue('debian');
		await page.getByTestId('cloudinit-user').fill('ubuntu');
		await page.getByTestId('cloudinit-ip-mode').selectOption('dhcp');
		await page.getByTestId('cloudinit-save').click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await expect(page.getByTestId('cloudinit-save-scopes')).toBeVisible();
		await expect(page.getByTestId('cloudinit-reboot-checkbox')).not.toBeChecked();
		// The form already shows the typed value, so wait for the PUT itself:
		// reloading while it is in flight aborts the save.
		const saved = page.waitForResponse(
			(response) => response.request().method() === 'PUT' && response.url().endsWith('/vms/default/102/cloudinit')
		);
		await page.getByTestId('cloudinit-save-confirm').click();
		expect((await saved).status()).toBe(200);
		await expect(page.getByRole('dialog')).toBeHidden();
		await expect(page.getByTestId('cloudinit-user')).toHaveValue('ubuntu');
		await page.reload();
		await page.getByTestId('vm-tab-configuration').click();
		await page.getByTestId('vm-tab-cloudinit').click();
		await expect(page.getByTestId('cloudinit-user')).toHaveValue('ubuntu');
		await expect(page.getByTestId('cloudinit-ip-mode')).toHaveValue('dhcp');
	});

	test('offers admin templates only - no YAML editor for users', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/102');
		await page.getByTestId('vm-tab-configuration').click();
		await page.getByTestId('vm-tab-cloudinit').click();
		await page.getByTestId('cloudinit-mode-document').click();
		await expect(page.getByTestId('cloudinit-document')).toBeVisible();
		await expect(page.locator('textarea')).toHaveCount(0);

		// The API refuses user-authored YAML outright.
		const response = await page.request.put('/api/v1/vms/default/102/cloudinit/document', {
			data: { content: '#cloud-config\nusers: {}\n' }
		});
		expect(response.status()).toBeGreaterThanOrEqual(400);
	});

	test('reboot checkbox uses server-side T05 reboot and denies non-owner access', async ({ page, request }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/102');
		await page.getByTestId('vm-tab-configuration').click();
		await page.getByTestId('vm-tab-cloudinit').click();
		await page.getByTestId('cloudinit-user').fill('debian');
		await page.getByTestId('cloudinit-save').click();
		await page.getByTestId('cloudinit-reboot-checkbox').check();
		await page.getByTestId('cloudinit-save-confirm').click();
		await expect(page.getByTestId('vm-status')).toContainText(/running/i);

		await signIn(request, 'bob', 'pvmss-bob');
		const response = await request.get('/api/v1/vms/default/102/cloudinit');
		expect(response.status()).toBe(403);
		expect(await response.json()).toEqual({ code: 'forbidden', message: 'not your VM' });
	});
});
