import { test, expect, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

// The Playwright server process holds the fake dataset in memory across test
// files, so VMs created here must be deleted again - T04's list specs assert
// exact row counts.
async function deleteCreatedVms(request: APIRequestContext): Promise<void> {
	// cluster is required: the all-clusters path (empty cluster) fails
	// server-side, which silently skipped this cleanup.
	const list = await request.get('/api/v1/vms?cluster=default&pageSize=100');
	if (!list.ok()) return;
	const vms = (await list.json()) as { items: { vmid: number; name: string }[] };
	for (const vm of vms.items) {
		if (vm.name.startsWith('web-e2e-')) {
			// force=true: these VMs are created running, and a plain delete is
			// refused with 409 for a running VM, which left them behind.
			await request.delete(`/api/v1/vms/default/${vm.vmid}?force=true`, { headers: await csrfHeaders(request) });
		}
	}
}

test.describe('T06 VM creation', () => {
	test.afterEach(async ({ page }) => {
		// Re-authenticate as alice: the last test in this file signs in as
		// admin, who cannot delete a pool-owned VM.
		await signInAlice(page.request);
		await deleteCreatedVms(page.request);
	});
	test('a fresh visit opens on the mode chooser, and Simple enters the guided form', async ({
		page
	}) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/vms/create');

		// The chooser asks the question; neither wizard is rendered yet.
		await expect(page.getByRole('heading', { name: 'Create a VM' })).toBeVisible();
		await expect(page.getByText('Creation mode')).toBeVisible();
		await expect(page.getByRole('button', { name: /Simple/ })).toBeVisible();
		await expect(page.getByRole('button', { name: /Detailed/ })).toBeVisible();
		await expect(page.getByLabel('Name')).toHaveCount(0);

		await page.getByRole('button', { name: /Simple/ }).click();
		await expect(page.getByLabel('Name')).toBeVisible();
		// The old mode tab row is gone - Detailed is only reachable via the chooser.
		await expect(page.getByRole('tab', { name: 'Detailed' })).toHaveCount(0);
	});

	test('change mode returns to the chooser with the entered values kept', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/vms/create');

		await page.getByRole('button', { name: /Simple/ }).click();
		await page.getByLabel('Name').fill('web-e2e-change');

		await page.getByTestId('vm-create-change-mode').click();
		// Back on the chooser: the cards are there and the form is gone.
		await expect(page.getByRole('button', { name: /Detailed/ })).toBeVisible();
		await expect(page.getByLabel('Name')).toHaveCount(0);

		// Re-entering the same mode keeps what was typed.
		await page.getByRole('button', { name: /Simple/ }).click();
		await expect(page.getByLabel('Name')).toHaveValue('web-e2e-change');
	});

	test('detailed mode: the step row reads as a stepper and moves', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/vms/create');
		await page.getByRole('button', { name: /Detailed/ }).click();

		const base = page.getByRole('tab', { name: 'Base' });
		await expect(base).toHaveAttribute('aria-current', 'step');

		await page.getByRole('tab', { name: 'Disk' }).click();
		await expect(page.getByRole('tab', { name: 'Disk' })).toHaveAttribute('aria-current', 'step');
		await expect(base).not.toHaveAttribute('aria-current', 'step');
	});

	test('simple mode: create a VM and watch it appear in the list', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		await page.getByRole('link', { name: 'Create a VM' }).click();
		await expect(page).toHaveURL(/\/vms\/create/);

		await page.getByRole('button', { name: /Simple/ }).click();
		await page.getByRole('radio', { name: /Medium/ }).check();
		await page.getByLabel('Name').fill('web-e2e-01');
		await page.getByRole('button', { name: 'Create VM' }).click();

		await expect(page).toHaveURL(/\/vms$/);

		// The navbar task tray was replaced by the Activity badge + Toaster in
		// the Layer B shell, so the completion signal is the toast.
		await expect(page.getByText('VM "web-e2e-01" created')).toBeVisible({ timeout: 20000 });

		// The redirect lands on the all-clusters list, whose first page (10
		// rows) does not reach a name sorting this late, so filter by cluster.
		await page.goto('/vms?cluster=default');
		const row = page.locator('[data-testid="vm-row"]', { hasText: 'web-e2e-01' });
		await expect(row).toBeVisible({ timeout: 20000 });
		await expect(row).toContainText(/running/i);
	});

	test('simple mode: an empty name shows an inline error and blocks submit', async ({ page }) => {
		await signInAlice(page.request);
		// Locale defaults to fr (locale.svelte.ts DEFAULT_LOCALE) unless a
		// preference is stored - force en so the assertions below are
		// deterministic regardless of the host's default.
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await page.goto('/vms/create');

		await page.getByRole('button', { name: /Simple/ }).click();
		await page.getByRole('radio', { name: /Medium/ }).check();

		// The name is empty, so the inline error is already rendered and
		// submit is disabled - the form cannot silently "do nothing".
		await expect(page.getByText('Name is required.')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Create VM' })).toBeDisabled();
		await expect(page).toHaveURL(/\/vms\/create$/);

		// A valid name clears the error and re-enables submit.
		await page.getByLabel('Name').fill('web-e2e-emptyname');
		await expect(page.getByText('Name is required.')).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'Create VM' })).toBeEnabled();
	});

	test('detailed mode: explicit node/storage/bridge create the exact VM', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/vms/create');

		await page.getByRole('button', { name: /Detailed/ }).click();
		await page.getByLabel('Name').fill('web-e2e-02');
		await page.getByLabel('Node').selectOption('pve-node-02');
		await page.getByRole('tab', { name: 'Disk' }).click();
		await page.getByLabel('Storage').selectOption('ceph-data');
		await page.getByRole('tab', { name: 'Network' }).click();
		// vmbr2 is the bridge approved on pve-node-02; vmbr0/vmbr1 live on
		// pve-node-01, so they are not offered for this node.
		await page.getByLabel('Bridge').selectOption('vmbr2');

		await page.getByRole('tab', { name: 'Review' }).click();

		const outgoing = await page.locator('[data-testid="review-request"]').textContent();
		expect(outgoing).toContain('"node": "pve-node-02"');
		expect(outgoing).toContain('"bridge": "vmbr2"');
		expect(outgoing).not.toContain('profileId');

		await page.getByRole('button', { name: 'Create VM' }).click();
		await expect(page).toHaveURL(/\/vms$/);
		await expect(page.getByText('VM "web-e2e-02" created')).toBeVisible({ timeout: 20000 });
	});

	test('no draft: reloading mid-fill starts fresh', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await signInAlice(page.request);
		await page.goto('/vms/create');

		await page.getByRole('button', { name: /Detailed/ }).click();
		await page.getByLabel('Name').fill('web-e2e-nodraft');

		await page.reload();

		// Nothing is persisted, so the reload comes back empty.
		await expect(page.getByText(/Draft saved|will be restored/)).toHaveCount(0);
		await page.getByRole('button', { name: /Detailed/ }).click();
		await expect(page.getByLabel('Name')).toHaveValue('');
	});

	test('catalog enforcement: a direct API call outside the catalog is rejected', async ({ page }) => {
		await signInAlice(page.request);

		// SC-004: no UI dropdown involved - a raw request with an unapproved storage.
		const response = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: {
				cluster: 'default',
				name: 'web-e2e-03',
				node: 'pve-node-01',
				cpuCores: 1,
				memoryMB: 1024,
				disk: { storage: 'not-a-real-storage', sizeGB: 20 },
				network: [{ bridge: 'vmbr0', model: 'virtio' }]
			}
		});
		expect(response.status()).toBe(400);
		const body = (await response.json()) as { code: string };
		expect(body.code).toBe('not_approved');
	});

	test('pool non-forgeability: a forged pool field has no effect', async ({ page }) => {
		await signInAlice(page.request);

		// SC-003: strict decoding rejects the unknown field outright; either
		// way no VM is created with a pool other than alice's.
		const response = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: {
				cluster: 'default',
				name: 'web-e2e-04',
				profileId: 'small',
				pool: 'pool-bob'
			}
		});
		expect([202, 400]).toContain(response.status());
		if (response.status() === 202) {
			const created = (await response.json()) as { vmid: number };
			const list = await page.request.get('/api/v1/vms?scope=all&pageSize=100');
			const vms = (await list.json()) as { items: { vmid: number; pool: string }[] };
			const createdVm = vms.items.find((item) => item.vmid === created.vmid);
			// The Index may not reflect it yet (task still running); the pool
			// check happens in the Go suite - here it suffices that creation did
			// not adopt the forged value when observable.
			if (createdVm !== undefined) {
				expect(createdVm.pool).toBe('pool-alice');
			}
		}
	});

	test('admin cannot create a VM: the self-service portal requires a pool', async ({ page }) => {
		const login = await page.request.post('/api/v1/auth/admin-login', {
			data: { password: 'pvmss-e2e-admin' }
		});
		expect(login.status()).toBe(200);

		const response = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: {
				cluster: 'default',
				name: 'web-e2e-admin-01',
				profileId: 'small'
			}
		});
		// VM ownership requires a personal pool, which admins do not have
		// (vm/create.go: ErrAdminCannotCreate).
		expect(response.status()).toBe(403);
		expect((await response.json()).code).toBe('admin_cannot_create');
	});
});
