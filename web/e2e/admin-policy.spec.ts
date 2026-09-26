import { expect, test, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './support/csrf';
import { deleteVmsByPrefix } from './support/vms';

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	expect(response.status()).toBe(200);
}

async function savePolicy(request: APIRequestContext, body: object): Promise<void> {
	const response = await request.put('/api/v1/admin/policy', {
		headers: await csrfHeaders(request),
		data: { cluster: 'default', ...body }
	});
	expect(response.status()).toBe(200);
}

test.describe('T12 admin policy', () => {
	test.describe.configure({ mode: 'serial' });

	// Policy is global server state. Restore the defaults after the file so a
	// failure part-way through a test cannot leave a lowered gabarit or quota
	// behind and block every later spec's VM creation. The capacity test also
	// creates VMs through the API; remove them so they do not inflate the
	// counts vm-list asserts on.
	test.afterAll(async ({ request }) => {
		await signInAdmin(request);
		await savePolicy(request, { gabarit: { maxDiskPerVmGb: 500, allowCustomYaml: true } });
		await savePolicy(request, { quota: { maxVmPerUser: -1 } });
		// The capacity test lowers pve-node-02's vCPU ceiling; lift it even if
		// the test failed before its own reset.
		await request.put('/api/v1/admin/policy/nodes/pve-node-02', {
			headers: await csrfHeaders(request),
			data: { cluster: 'default', maxVcpus: 0 }
		});
		await signInAlice(request);
		await deleteVmsByPrefix(request, 'capacity-demo');
	});

	test('admin lowers a gabarit and creation is refused before a task is accepted', async ({ page }) => {
		await signInAdmin(page.request);
		await page.goto('/admin/policy');
		await page.getByLabel('Maximum disk per VM (GB)').fill('10');
		await page.getByRole('button', { name: 'Save policy' }).click();
		// Target the toast: the page also renders the cluster-status banner with
		// role="status", so a bare getByRole('status') is ambiguous.
		await expect(page.getByTestId('toast').filter({ hasText: 'Policy saved' }).first()).toBeVisible();

		await signInAlice(page.request);
		const response = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: { cluster: 'default', name: 'policy-gabarit-demo', profileId: 'small' }
		});
		expect(response.status()).toBe(400);
		expect((await response.json()).code).toBe('gabarit_exceeded');

		// Restore straight away - the lowered gabarit blocks every later create.
		await signInAdmin(page.request);
		await savePolicy(page.request, { gabarit: { maxDiskPerVmGb: 500 } });
	});

	test('admin quota is reflected by list and rejects the next creation', async ({ page }) => {
		await signInAdmin(page.request);
		await savePolicy(page.request, { quota: { maxVmPerUser: 1 } });
		await signInAlice(page.request);
		const response = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: { cluster: 'default', name: 'policy-quota-demo', profileId: 'small' }
		});
		expect(response.status()).toBe(400);
		expect((await response.json()).code).toBe('quota_exceeded');
		await signInAdmin(page.request);
		await savePolicy(page.request, { quota: { maxVmPerUser: -1 } });
	});

	test('node capacity is enforced for creation and hardware growth', async ({ page }) => {
		await signInAdmin(page.request);
		// Load the storages and bridges pages first: they run discovery, and a
		// resource that has not been discovered yet cannot be toggled (404).
		await page.goto('/admin/storages');
		await page.goto('/admin/bridges');
		// pve-node-02 is online and carries ceph-data (images) and vmbr2 in the
		// fake dataset; pve-node-03 is offline and has no VM-capable storage.
		for (const [path, body] of [
			['/api/v1/admin/nodes/toggle', { cluster: 'default', name: 'pve-node-02', enabled: true }],
			['/api/v1/admin/storages/toggle', { cluster: 'default', name: 'ceph-data', node: 'pve-node-02', enabled: true }],
			['/api/v1/admin/bridges/toggle', { cluster: 'default', name: 'vmbr2', node: 'pve-node-02', enabled: true }]
		] as const) {
			const response = await page.request.post(path, {
				headers: await csrfHeaders(page.request),
				data: body
			});
			expect(response.status(), path).toBe(200);
		}
		await page.goto('/admin/policy/nodes');
		await expect(page.getByRole('heading', { name: 'Node capacity' })).toBeVisible();
		const nodesResponse = await page.request.get('/api/v1/admin/policy/nodes?cluster=default');
		expect(nodesResponse.status()).toBe(200);
		const nodes = (await nodesResponse.json()) as Array<{ node: string; usedVcpus: number }>;
		const node = nodes.find((item) => item.node === 'pve-node-02');
		expect(node).toBeDefined();
		const capacityResponse = await page.request.put('/api/v1/admin/policy/nodes/pve-node-02', {
			headers: await csrfHeaders(page.request),
			data: { cluster: 'default', maxVcpus: (node?.usedVcpus ?? 0) + 1 }
		});
		expect(capacityResponse.status()).toBe(200);
		await signInAlice(page.request);
		const first = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: {
				cluster: 'default', name: 'capacity-demo-one', node: 'pve-node-02', cpuCores: 1, memoryMB: 1024,
				disk: { storage: 'ceph-data', sizeGB: 10 }, network: [{ bridge: 'vmbr2', model: 'virtio' }]
			}
		});
		expect(first.status()).toBe(202);
		const accepted = (await first.json()) as { vmid: number; upid: string };
		for (let attempt = 0; attempt < 3; attempt += 1) {
			const task = await page.request.get(`/api/v1/tasks/${encodeURIComponent(accepted.upid)}?cluster=default`);
			expect(task.status()).toBe(200);
			if ((await task.json()).state === 'ok') break;
		}
		const second = await page.request.post('/api/v1/vms', {
			headers: await csrfHeaders(page.request),
			data: {
				cluster: 'default', name: 'capacity-demo-two', node: 'pve-node-02', cpuCores: 1, memoryMB: 1024,
				disk: { storage: 'ceph-data', sizeGB: 10 }, network: [{ bridge: 'vmbr2', model: 'virtio' }]
			}
		});
		expect(second.status()).toBe(400);
		expect((await second.json()).code).toBe('capacity_exceeded');
		const hardware = await page.request.put(`/api/v1/vms/default/${accepted.vmid}/hardware`, {
			headers: await csrfHeaders(page.request),
			data: { sockets: 1, cores: 4, memoryMB: 1024 }
		});
		expect(hardware.status()).toBe(400);
		expect((await hardware.json()).code).toBe('capacity_exceeded');
		await signInAdmin(page.request);
		const reset = await page.request.put('/api/v1/admin/policy/nodes/pve-node-02', {
			headers: await csrfHeaders(page.request),
			data: { cluster: 'default', maxVcpus: 0 }
		});
		expect(reset.status()).toBe(200);
	});
});
