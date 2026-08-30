import { test, expect, type APIRequestContext } from '@playwright/test';

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

// The Playwright server process holds the fake dataset in memory across test
// files, so VMs created here must be deleted again — T04's list specs assert
// exact row counts.
async function deleteCreatedVms(request: APIRequestContext): Promise<void> {
	const list = await request.get('/api/v1/vms?scope=all&pageSize=100');
	if (!list.ok()) return;
	const vms = (await list.json()) as { items: { vmid: number; name: string }[] };
	for (const vm of vms.items) {
		if (vm.name.startsWith('web-e2e-')) {
			await request.delete(`/api/v1/vms/default/${vm.vmid}`);
		}
	}
}

test.describe('T06 VM creation', () => {
	test.afterEach(async ({ page }) => {
		await deleteCreatedVms(page.request);
	});
	test('simple mode: create a VM and watch the task complete in the tray', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms?cluster=default');

		await page.getByRole('link', { name: 'Create a VM' }).click();
		await expect(page).toHaveURL(/\/vms\/create/);

		await page.getByRole('radio', { name: /Medium/ }).check();
		await page.getByLabel('Name').fill('web-e2e-01');
		await page.getByRole('button', { name: 'Create VM' }).click();

		await expect(page).toHaveURL(/\/vms$/);
		await expect(page.getByRole('status', { name: /task\(s\) in progress/ })).toBeVisible();

		await expect(page.getByText('VM "web-e2e-01" created')).toBeVisible({ timeout: 20000 });
		await expect(page.getByRole('status', { name: /task\(s\) in progress/ })).toHaveCount(0);

		const row = page.locator('[data-testid="vm-row"]', { hasText: 'web-e2e-01' });
		await expect(row).toBeVisible({ timeout: 20000 });
		await expect(row).toContainText('running');
	});

	test('simple mode: submitting with an empty name shows an inline error instead of doing nothing', async ({ page }) => {
		await signInAlice(page.request);
		// Locale defaults to fr (locale.svelte.ts DEFAULT_LOCALE) unless a
		// preference is stored — force en so the assertions below are
		// deterministic regardless of the host's default.
		await page.addInitScript(() => localStorage.setItem('pvmss-locale', 'en'));
		await page.goto('/vms/create');

		await page.getByRole('radio', { name: /Medium/ }).check();
		await page.getByRole('button', { name: 'Create VM' }).click();

		await expect(page.getByText('Name is required.')).toBeVisible();
		await expect(page).toHaveURL(/\/vms\/create$/);
	});

	test('detailed mode: explicit node/storage/bridge create the exact VM', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/create');

		await page.getByRole('tab', { name: 'Detailed' }).click();
		await page.getByLabel('Name').fill('web-e2e-02');
		await page.getByLabel('Node').selectOption('pve-node-02');
		await page.getByRole('tab', { name: 'Disk' }).click();
		await page.getByLabel('Storage').selectOption('ceph-data');
		await page.getByRole('tab', { name: 'Network' }).click();
		await page.getByLabel('Bridge').selectOption('vmbr1');

		await page.getByRole('tab', { name: 'Review' }).click();

		const outgoing = await page.locator('[data-testid="review-request"]').textContent();
		expect(outgoing).toContain('"node": "pve-node-02"');
		expect(outgoing).toContain('"bridge": "vmbr1"');
		expect(outgoing).not.toContain('profileId');

		await page.getByRole('button', { name: 'Create VM' }).click();
		await expect(page).toHaveURL(/\/vms$/);
		await expect(page.getByText('VM "web-e2e-02" created')).toBeVisible({ timeout: 20000 });
	});

	test('draft: reloading mid-fill restores the values with a toast', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/create');

		await page.getByRole('tab', { name: 'Detailed' }).click();
		await page.getByLabel('Name').fill('web-e2e-draft');
		await page.getByLabel('Tags').fill('team-web');

		await page.waitForTimeout(700);
		await page.reload();

		await expect(page.getByText(/Draft restored/)).toBeVisible();
		await expect(page.getByLabel('Name')).toHaveValue('web-e2e-draft');
		// The mode was part of the draft — we land back on the detailed wizard.
		await expect(page.getByRole('tab', { name: 'Detailed' })).toHaveAttribute('aria-selected', 'true');
	});

	test('catalog enforcement: a direct API call outside the catalog is rejected', async ({ page }) => {
		await signInAlice(page.request);

		// SC-004: no UI dropdown involved — a raw request with an unapproved storage.
		const response = await page.request.post('/api/v1/vms', {
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
			// check happens in the Go suite — here it suffices that creation did
			// not adopt the forged value when observable.
			if (createdVm !== undefined) {
				expect(createdVm.pool).toBe('pool-alice');
			}
		}
	});

	test('admin without pool can create a VM', async ({ page }) => {
		const login = await page.request.post('/api/v1/auth/admin-login', {
			data: { password: 'pvmss-e2e-admin' }
		});
		expect(login.status()).toBe(200);

		const response = await page.request.post('/api/v1/vms', {
			data: {
				cluster: 'default',
				name: 'web-e2e-admin-01',
				profileId: 'small'
			}
		});
		expect(response.status()).toBe(202);

		const created = (await response.json()) as { vmid: number; name: string };
		expect(created.name).toBe('web-e2e-admin-01');
		expect(created.vmid).toBeGreaterThan(0);
	});
});
