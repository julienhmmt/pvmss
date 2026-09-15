import { test, expect, type APIRequestContext } from '@playwright/test';

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

async function signInBob(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'bob', password: 'pvmss-bob', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

const fileContent = '#cloud-config\npackages:\n  - htop\n';

test.describe('user cloud-init files', () => {
	test.describe.configure({ mode: 'serial' });

	test('alice manages her files at /cloud-init; bob never sees them', async ({ page }) => {
		// Default locale is French; pin English so the English selectors match.
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/cloud-init');

		// Empty state first.
		await expect(page.getByTestId('cloudinit-files-empty')).toBeVisible();

		// Create.
		await page.getByRole('button', { name: 'New file' }).first().click();
		await page.getByLabel('Label').fill('Dev box');
		await page.getByLabel('Content').fill(fileContent);
		await page.getByRole('button', { name: 'Create' }).click();

		const row = page.locator('tr', { hasText: 'Dev box' });
		await expect(row).toBeVisible();
		await expect(row.locator('td.font-mono')).toHaveText('dev-box');

		// Edit the label - dialog opens with the stored content.
		await row.getByRole('button', { name: 'Edit Dev box' }).click();
		await expect(page.getByLabel('Content')).toHaveValue(fileContent);
		await page.getByLabel('Label').fill('Dev box v2');
		await page.getByRole('button', { name: 'Save' }).click();
		await expect(page.locator('tr', { hasText: 'Dev box v2' })).toBeVisible();

		// Owner isolation: bob's list is empty and the id 404s for him.
		await signInBob(page.request);
		await page.goto('/cloud-init');
		await expect(page.getByTestId('cloudinit-files-empty')).toBeVisible();
		const bobGet = await page.request.get('/api/v1/cloudinit/files/dev-box');
		expect(bobGet.status()).toBe(404);

		// Back to alice: delete via the confirm dialog → empty state.
		await signInAlice(page.request);
		await page.goto('/cloud-init');
		const updatedRow = page.locator('tr', { hasText: 'Dev box v2' });
		await expect(updatedRow).toBeVisible();
		await updatedRow.getByRole('button', { name: 'Delete Dev box v2' }).click();
		await page.getByTestId('cloudinit-file-delete-confirm').click();
		await expect(page.getByTestId('cloudinit-files-empty')).toBeVisible();
	});
});
