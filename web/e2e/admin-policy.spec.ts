import { expect, test, type APIRequestContext } from '@playwright/test';

async function signInAdmin(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(response.status()).toBe(200);
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', { data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' } });
	expect(response.status()).toBe(200);
}

async function savePolicy(request: APIRequestContext, body: object): Promise<void> {
	const response = await request.put('/api/v1/admin/policy', { data: { cluster: 'default', ...body } });
	expect(response.status()).toBe(200);
}

test.describe('T12 admin policy', () => {
	test.describe.configure({ mode: 'serial' });

	test('admin lowers a gabarit and creation is refused before a task is accepted', async ({ page }) => {
		await signInAdmin(page.request);
		await page.goto('/admin/policy');
		await page.getByLabel('Maximum disk per VM (GB)').fill('10');
		await page.getByRole('button', { name: 'Save policy' }).click();
		await expect(page.getByRole('status')).toContainText('saved');
		await signInAlice(page.request);
		const response = await page.request.post('/api/v1/vms', { data: { cluster: 'default', name: 'policy-gabarit-demo', profileId: 'small' } });
		expect(response.status()).toBe(400);
		expect((await response.json()).code).toBe('gabarit_exceeded');
		await signInAdmin(page.request);
		await savePolicy(page.request, { gabarit: { maxDiskPerVmGb: 500, allowCustomYaml: false } });
		await signInAlice(page.request);
		const snippet = await page.request.put('/api/v1/vms/default/101/cloudinit/snippet', { data: { content: 'not yaml' } });
		expect(snippet.status()).toBe(403);
		expect((await snippet.json()).code).toBe('custom_yaml_disabled');
		await signInAdmin(page.request);
		await savePolicy(page.request, { gabarit: { maxDiskPerVmGb: 500, allowCustomYaml: true } });
	});

	test('admin quota is reflected by list and rejects the next creation', async ({ page }) => {
		await signInAdmin(page.request);
		await savePolicy(page.request, { quota: { maxVmPerUser: 1 } });
		await signInAlice(page.request);
		const response = await page.request.post('/api/v1/vms', { data: { cluster: 'default', name: 'policy-quota-demo', profileId: 'small' } });
		expect(response.status()).toBe(400);
		expect((await response.json()).code).toBe('quota_exceeded');
		await signInAdmin(page.request);
		await savePolicy(page.request, { quota: { maxVmPerUser: -1 } });
	});

	test('node capacity is enforced for creation and hardware growth', async ({ page }) => {
		await signInAdmin(page.request);
		for (const body of [
			{ cluster: 'default', name: 'pve-node-03', enabled: true },
			{ cluster: 'default', name: 'backup-nfs', node: 'pve-node-03', enabled: true }
		]) {
			const path = 'node' in body ? '/api/v1/admin/storages/toggle' : '/api/v1/admin/nodes/toggle';
			const response = await page.request.post(path, { data: body });
			expect(response.status()).toBe(200);
		}
		await page.goto('/admin/policy/nodes');
		await expect(page.getByRole('heading', { name: 'Node capacity' })).toBeVisible();
		const nodesResponse = await page.request.get('/api/v1/admin/policy/nodes?cluster=default');
		expect(nodesResponse.status()).toBe(200);
		const nodes = (await nodesResponse.json()) as Array<{ node: string; usedVcpus: number }>;
		const node = nodes.find((item) => item.node === 'pve-node-03');
		expect(node).toBeDefined();
		const capacityResponse = await page.request.put('/api/v1/admin/policy/nodes/pve-node-03', {
			data: { cluster: 'default', maxVcpus: (node?.usedVcpus ?? 0) + 1 }
		});
		expect(capacityResponse.status()).toBe(200);
		await signInAlice(page.request);
		const first = await page.request.post('/api/v1/vms', { data: {
			cluster: 'default', name: 'capacity-demo-one', node: 'pve-node-03', cpuCores: 1, memoryMB: 1024,
			disk: { storage: 'backup-nfs', sizeGB: 10 }, network: { bridge: 'vmbr0', model: 'virtio' }
		} });
		expect(first.status()).toBe(202);
		const accepted = (await first.json()) as { vmid: number; upid: string };
		for (let attempt = 0; attempt < 3; attempt += 1) {
			const task = await page.request.get(`/api/v1/tasks/${encodeURIComponent(accepted.upid)}`);
			expect(task.status()).toBe(200);
			if ((await task.json()).state === 'ok') break;
		}
		const second = await page.request.post('/api/v1/vms', { data: {
			cluster: 'default', name: 'capacity-demo-two', node: 'pve-node-03', cpuCores: 1, memoryMB: 1024,
			disk: { storage: 'backup-nfs', sizeGB: 10 }, network: { bridge: 'vmbr0', model: 'virtio' }
		} });
		expect(second.status()).toBe(400);
		expect((await second.json()).code).toBe('capacity_exceeded');
		const hardware = await page.request.put(`/api/v1/vms/default/${accepted.vmid}/hardware`, { data: { sockets: 1, cores: 4, memoryMB: 1024 } });
		expect(hardware.status()).toBe(400);
		expect((await hardware.json()).code).toBe('capacity_exceeded');
		await signInAdmin(page.request);
		const reset = await page.request.put('/api/v1/admin/policy/nodes/pve-node-03', { data: { cluster: 'default', maxVcpus: 0 } });
		expect(reset.status()).toBe(200);
	});
});
