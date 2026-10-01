import { test, expect, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './support/csrf';
import { deleteVmsByPrefix } from './support/vms';

// Two distinct, valid ed25519 public keys - the profile API parses them, so
// the placeholder strings the injection spec uses would not pass validation.
const PROFILE_KEY =
	'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDlZOR9zbKxKuE3LMl97SyEXgFJdkFn4XBqilMDVomuO laptop@alice';
const PASTED_KEY =
	'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMdGdRMZ82PZ429b6tcin/FauyMariv0VAAlGqz/aSBY desktop@alice';

const PROFILE_LABEL = 'e2e-laptop-key';
const PASTED_LABEL = 'e2e-desktop-key';
// Deliberately outside vm-create.spec's "web-e2e-" sweep prefix - under
// fullyParallel its afterEach would otherwise delete these VMs mid-test.
const VM_PREFIX = 'e2e-key';
const IMAGE_VALUE = 'local|ubuntu-24.04-server-cloudimg-amd64.qcow2';

interface ProfileKey {
	id: string;
	label: string;
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

async function listProfileKeys(request: APIRequestContext): Promise<ProfileKey[]> {
	const response = await request.get('/api/v1/profile/ssh-keys');
	expect(response.status()).toBe(200);
	return ((await response.json()) as { keys: ProfileKey[] }).keys;
}

// The profile-key rows live in the SQLite file, which survives between runs:
// wiping alice's keys before and after each test keeps the spec independent
// of leftovers from an interrupted run.
async function deleteAllProfileKeys(request: APIRequestContext): Promise<void> {
	for (const key of await listProfileKeys(request)) {
		const response = await request.delete(`/api/v1/profile/ssh-keys/${key.id}`, {
			headers: await csrfHeaders(request)
		});
		expect([204, 404]).toContain(response.status());
	}
}

// The ubuntu cloud image ships enabled (schemaV27 seed) but a prior run or
// an admin spec can toggle or delete the row; the toggle endpoint upserts,
// so this restores image mode deterministically. Bridge approvals are empty on
// a fresh database (schemaV14 recreates the table), and image mode creates on
// the image's node, so a bridge there is approved the same way - without it
// the spec only passes against a database another run already populated.
async function ensureCloudImage(request: APIRequestContext): Promise<void> {
	await signInAdmin(request);
	const bridge = await request.post('/api/v1/admin/bridges/toggle', {
		headers: await csrfHeaders(request),
		data: { cluster: 'default', node: 'pve-node-01', name: 'vmbr0', enabled: true }
	});
	expect(bridge.status()).toBe(200);
	const response = await request.post('/api/v1/admin/images/toggle', {
		headers: await csrfHeaders(request),
		data: {
			cluster: 'default',
			node: 'pve-node-01',
			storage: 'local',
			file: 'ubuntu-24.04-server-cloudimg-amd64.qcow2',
			enabled: true
		}
	});
	expect(response.status()).toBe(200);
	await signInAlice(request);
}

test.describe('profile SSH keys', () => {
	test.describe.configure({ mode: 'serial' });

	test.beforeEach(async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await deleteAllProfileKeys(page.request);
		await deleteVmsByPrefix(page.request, VM_PREFIX);
		await ensureCloudImage(page.request);
	});

	test.afterEach(async ({ page }) => {
		await signInAlice(page.request);
		await deleteAllProfileKeys(page.request);
		await deleteVmsByPrefix(page.request, VM_PREFIX);
	});

	test('a saved profile key is pre-picked in the create wizard and lands on the VM', async ({
		page
	}) => {
		await page.goto('/profile');
		await page.getByTestId('ssh-key-add-label').fill(PROFILE_LABEL);
		await page.getByTestId('ssh-key-add-key').fill(PROFILE_KEY);
		await page.getByTestId('ssh-key-add').click();
		const section = page.getByTestId('profile-ssh-keys');
		await expect(section.getByText(PROFILE_LABEL)).toBeVisible();

		const key = (await listProfileKeys(page.request)).find((entry) => entry.label === PROFILE_LABEL);
		expect(key).toBeDefined();

		await page.goto('/vms/create');
		await page.getByRole('button', { name: /Simple/ }).click();
		// The card's invisible radio overlays its content, so check the input
		// directly rather than clicking the label text.
		await page.locator('input[name="simple-source"][value="image"]').check();
		await page.getByRole('combobox', { name: /Cloud image/ }).selectOption(IMAGE_VALUE);
		await page.getByRole('radio', { name: /Small/ }).check();

		// One profile key means default-selected (3 or fewer selects all).
		const option = page.getByTestId(`ssh-key-picker-option-${key!.id}`);
		await expect(option.getByRole('checkbox')).toBeChecked();

		await page.getByRole('textbox', { name: 'Name', exact: true }).fill(`${VM_PREFIX}-a`);
		await page.getByLabel('Username').fill('alice');
		await page.getByRole('button', { name: 'Create this machine' }).click();

		await expect(page).toHaveURL(/\/vms$/);
		await expect(page.getByText(`VM "${VM_PREFIX}-a" created`)).toBeVisible({ timeout: 20000 });

		await page.goto('/vms?cluster=default');
		await expect(
			page.locator('[data-testid="vm-row"]', { hasText: `${VM_PREFIX}-a` })
		).toBeVisible({ timeout: 20000 });
	});

	test('an unknown pasted key is offered and saved to the profile after create', async ({
		page
	}) => {
		await page.goto('/vms/create');
		await page.getByRole('button', { name: /Simple/ }).click();
		await page.locator('input[name="simple-source"][value="image"]').check();
		await page.getByRole('combobox', { name: /Cloud image/ }).selectOption(IMAGE_VALUE);
		await page.getByRole('radio', { name: /Small/ }).check();
		await page.getByRole('textbox', { name: 'Name', exact: true }).fill(`${VM_PREFIX}-b`);
		await page.getByLabel('Username').fill('alice');

		await page.getByTestId('ssh-key-picker-paste').fill(PASTED_KEY);
		const saveBox = page.getByTestId('ssh-key-picker-save');
		await expect(saveBox).toBeVisible();
		await expect(saveBox).toBeChecked();
		// The label is prefilled from the key comment; use a stable one.
		await page.getByTestId('ssh-key-picker-save-label').fill(PASTED_LABEL);

		await page.getByRole('button', { name: 'Create this machine' }).click();
		await expect(page).toHaveURL(/\/vms$/);
		await expect(page.getByText(`VM "${VM_PREFIX}-b" created`)).toBeVisible({ timeout: 20000 });

		// The profile save fires after the create is accepted, so poll the
		// API rather than assuming it has landed by the redirect.
		await expect
			.poll(async () => (await listProfileKeys(page.request)).some((entry) => entry.label === PASTED_LABEL))
			.toBe(true);

		await page.goto('/profile');
		await expect(page.getByTestId('profile-ssh-keys').getByText(PASTED_LABEL)).toBeVisible();
	});
});
