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
		await page.getByTestId('vm-tab-cloudinit').click();
		await expect(page.getByTestId('cloudinit-user')).toHaveValue('debian');
		await page.getByTestId('cloudinit-user').fill('ubuntu');
		await page.getByTestId('cloudinit-ip-mode').selectOption('dhcp');
		await page.getByTestId('cloudinit-save').click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await expect(page.getByText('next VM reboot')).toBeVisible();
		await expect(page.getByTestId('cloudinit-reboot-checkbox')).not.toBeChecked();
		await page.getByTestId('cloudinit-save-confirm').click();
		await expect(page.getByTestId('cloudinit-user')).toHaveValue('ubuntu');
		await page.reload();
		await page.getByTestId('vm-tab-cloudinit').click();
		await expect(page.getByTestId('cloudinit-user')).toHaveValue('ubuntu');
		await expect(page.getByTestId('cloudinit-ip-mode')).toHaveValue('dhcp');
	});

	test('saves, reloads, and explicitly clears custom YAML', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/102');
		await page.getByTestId('vm-tab-cloudinit').click();
		await page.getByTestId('cloudinit-mode-yaml').click();
		await expect(page.getByTestId('cloudinit-snippet-content')).toHaveValue('');
		await page.getByTestId('cloudinit-snippet-content').fill('#cloud-config\nusers: {}\n');
		await page.getByTestId('cloudinit-snippet-save').click();
		await expect(page.getByTestId('cloudinit-snippet-content')).toHaveValue('#cloud-config\nusers: {}\n');
		await page.reload();
		await page.getByTestId('vm-tab-cloudinit').click();
		await page.getByTestId('cloudinit-mode-yaml').click();
		await expect(page.getByTestId('cloudinit-snippet-content')).toHaveValue('#cloud-config\nusers: {}\n');
		await page.getByTestId('cloudinit-snippet-content').fill('');
		await page.getByTestId('cloudinit-snippet-save').click();
		await expect(page.getByTestId('cloudinit-snippet-content')).toHaveValue('');
	});

	test('reboot checkbox uses server-side T05 reboot and denies non-owner access', async ({ page, request }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/102');
		await page.getByTestId('vm-tab-cloudinit').click();
		await page.getByTestId('cloudinit-user').fill('debian');
		await page.getByTestId('cloudinit-save').click();
		await page.getByTestId('cloudinit-reboot-checkbox').check();
		await page.getByTestId('cloudinit-save-confirm').click();
		await expect(page.getByTestId('vm-status')).toContainText('running');

		await signIn(request, 'bob', 'pvmss-bob');
		const response = await request.get('/api/v1/vms/default/102/cloudinit');
		expect(response.status()).toBe(403);
		expect(await response.json()).toEqual({ code: 'forbidden', message: 'not your VM' });
	});
});
