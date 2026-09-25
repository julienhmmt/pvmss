import { expect, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './csrf';

// The fake dataset is shared by the whole run and the Go server keeps it in
// memory, so a spec that deletes or renames a fixture VM breaks every later
// spec that asserts the pristine dataset (vm-list counts, vm-hardware, ...).
// Specs that mutate a VM must create their own through this module instead.

export interface CreatedVm {
	vmid: number;
	name: string;
}

export interface CreateVmOptions {
	/** Start the VM as part of the create task (the fake folds it in). */
	start?: boolean;
}

// Tracked by VMID rather than name so a renamed VM is still cleaned up.
const createdVmids: number[] = [];

async function waitFor(
	check: () => Promise<boolean>,
	what: string,
	timeoutMs = 20000
): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	while (Date.now() < deadline) {
		if (await check()) return;
		await new Promise((resolve) => setTimeout(resolve, 200));
	}
	throw new Error(`${what} did not become ready within ${timeoutMs}ms`);
}

async function vmStatus(request: APIRequestContext, vmid: number): Promise<string | null> {
	const response = await request.get(`/api/v1/vms/default/${vmid}/status`);
	if (!response.ok()) return null;
	return ((await response.json()) as { status: string }).status;
}

/**
 * Creates a VM as the signed-in user and waits until it is usable. The payload
 * mirrors the simple wizard's: node, storage and bridge are left to server
 * auto-selection, which needs at least one approved bridge in the catalog
 * (`schemaV14` drops the seeded `catalog_bridges` rows, so the admin specs that
 * discover and approve resources must have run first).
 */
export async function createVm(
	request: APIRequestContext,
	name: string,
	options: CreateVmOptions = {}
): Promise<CreatedVm> {
	const response = await request.post('/api/v1/vms', {
		headers: await csrfHeaders(request),
		data: {
			cluster: 'default',
			name,
			profileId: 'small',
			startAfterCreate: options.start === true
		}
	});
	if (response.status() !== 202) {
		throw new Error(`create VM ${name}: ${response.status()} ${await response.text()}`);
	}
	const vm = (await response.json()) as CreatedVm;
	createdVmids.push(vm.vmid);

	await waitFor(async () => (await vmStatus(request, vm.vmid)) !== null, `VM ${vm.vmid}`);
	if (options.start === true) {
		await waitFor(async () => (await vmStatus(request, vm.vmid)) === 'running', `VM ${vm.vmid} start`);
	}
	return vm;
}

/**
 * Deletes every VM created through this module, so a failed test cannot leak
 * rows into another spec's counts. Tolerates VMs a test already deleted.
 */
export async function cleanupCreatedVms(request: APIRequestContext): Promise<void> {
	for (const vmid of createdVmids.splice(0)) {
		const response = await request.delete(`/api/v1/vms/default/${vmid}?force=true`, {
			headers: await csrfHeaders(request)
		});
		expect([200, 404]).toContain(response.status());
	}
}

/**
 * Deletes every VM whose name starts with `prefix`, for specs that create VMs
 * through the UI rather than through createVm. Also drops matching VMIDs from
 * this module's registry so cleanupCreatedVms does not retry them.
 */
export async function deleteVmsByPrefix(request: APIRequestContext, prefix: string): Promise<void> {
	// cluster=default is required: the all-clusters path (empty cluster) fails
	// server-side, and the suite only ever creates VMs on the default cluster.
	const list = await request.get('/api/v1/vms?cluster=default&pageSize=100');
	if (!list.ok()) return;
	const vms = (await list.json()) as { items: { vmid: number; name: string }[] };

	for (const vm of vms.items) {
		if (!vm.name.startsWith(prefix)) continue;
		const response = await request.delete(`/api/v1/vms/default/${vm.vmid}?force=true`, {
			headers: await csrfHeaders(request)
		});
		expect([200, 404]).toContain(response.status());
		const index = createdVmids.indexOf(vm.vmid);
		if (index !== -1) createdVmids.splice(index, 1);
	}
}
