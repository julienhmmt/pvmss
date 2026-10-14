import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick } from 'svelte';
import { setLocale } from '$lib/paraglide/runtime.js';
import VmActionBar from './VmActionBar.svelte';
import VmHardwareTab from './hardware/VmHardwareTab.svelte';
import AddDiskDialog from './disks/AddDiskDialog.svelte';
import ResizeDiskDialog from './disks/ResizeDiskDialog.svelte';
import EditNetworkInterfaceDialog from './network/EditNetworkInterfaceDialog.svelte';
import { VmDetailStore, type VmDetailEntity, type HardwareOptions } from './detail.svelte';
import { ToastRegion } from '$lib/shared/ui/toast.svelte';

let storeInstance: VmDetailStore;
const toast = new ToastRegion();

vi.mock('./detail.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('./detail.svelte')>();
	return { ...original, getVmDetailContext: () => storeInstance };
});

vi.mock('$lib/shared/ui/toast.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('$lib/shared/ui/toast.svelte')>();
	return { ...original, getToastContext: () => toast };
});

function sampleEntity(status: 'running' | 'stopped' = 'running'): VmDetailEntity {
	return {
		vmid: 101,
		name: 'web-01',
		status,
		node: 'pve-node-01',
		cluster: 'default',
		pool: 'pvmss-alice',
		sockets: 1,
		cores: 2,
		cpuCores: 2,
		memoryTotal: 2048 * 1024 * 1024,
		diskTotal: 20 * 1024 * 1024 * 1024,
		uptimeSeconds: 120,
		tags: ['pvmss'],
		ostype: 'l26',
		disks: [{ key: 'scsi0', bus: 'scsi', busIndex: 0, storage: 'local-lvm', sizeGB: 20, isBoot: true }],
		networkInterfaces: [
			{ index: 0, bridge: 'vmbr0', model: 'virtio', mac: 'BC:24:11:00:00:01', vlan: null, rateMbps: null, ipAddresses: ['10.0.0.10'] }
		]
	};
}

function sampleHardwareOptions(): HardwareOptions {
	return {
		storages: [
			{ node: 'pve-node-01', storage: 'local-lvm', type: 'lvmthin' },
			{ node: 'pve-node-02', storage: 'local-lvm', type: 'lvmthin' }
		],
		bridges: [
			{ node: 'pve-node-01', bridge: 'vmbr0' },
			{ node: 'pve-node-01', bridge: 'vmbr1' },
			{ node: 'pve-node-02', bridge: 'vmbr0' }
		],
		isos: [],
		tags: [],
		limits: {
			maxSockets: 2,
			maxCores: 8,
			maxMemoryMB: 16384,
			maxDiskPerVMGB: 100,
			maxNetworkCards: 4,
			remainingBusSlots: { scsi: 3, virtio: 0, sata: 2, ide: 1 }
		}
	};
}

describe('Ticket 10: VM detail power confirmation & sub-tab form hygiene', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it('VmActionBar asks for confirmation when Shutdown is clicked', async () => {
		storeInstance = new VmDetailStore('default', 101);
		storeInstance.entity = sampleEntity('running');
		const actionSpy = vi.spyOn(storeInstance, 'action').mockResolvedValue();

		mount(VmActionBar, { target: document.body });

		const shutdownBtn = document.querySelector<HTMLButtonElement>('[data-testid="vm-action-shutdown"]');
		expect(shutdownBtn).not.toBeNull();
		shutdownBtn!.click();
		flushSync();
		await tick();

		expect(actionSpy).not.toHaveBeenCalled();
		const dialog = document.querySelector('[data-testid="vm-action-confirm"]');
		expect(dialog).not.toBeNull();
	});

	it('EditNetworkInterfaceDialog filters bridges to the VM node without duplicate-key errors or node labels', async () => {
		storeInstance = new VmDetailStore('default', 101);
		storeInstance.entity = sampleEntity('running');
		storeInstance.hardwareOptions = sampleHardwareOptions();

		mount(EditNetworkInterfaceDialog, {
			target: document.body,
			props: {
				open: true,
				iface: storeInstance.entity.networkInterfaces![0]!
			}
		});
		flushSync();
		await tick();

		const select = document.querySelector<HTMLSelectElement>('[data-testid="edit-nic-bridge"]');
		expect(select).not.toBeNull();
		const options = [...select!.querySelectorAll('option')].filter((opt) => !opt.disabled);
		expect(options.map((opt) => opt.value)).toEqual(['vmbr0', 'vmbr1']);
		expect(options.map((opt) => opt.textContent)).toEqual(['vmbr0', 'vmbr1']);
	});

	it('VmHardwareTab renders FormField limit hints and blocks out-of-range values with inline errors', () => {
		storeInstance = new VmDetailStore('default', 101);
		storeInstance.entity = sampleEntity('stopped');
		storeInstance.hardwareOptions = sampleHardwareOptions();
		const updateSpy = vi.spyOn(storeInstance, 'updateHardware').mockResolvedValue();

		mount(VmHardwareTab, { target: document.body });
		flushSync();

		expect(document.body.textContent).toContain('1–2 sockets allowed');
		expect(document.body.textContent).toContain('1–8 vCPU allowed');
		expect(document.body.textContent).toContain('128–16384 MB allowed');

		const coresInput = document.querySelector<HTMLInputElement>('[data-testid="vm-hardware-cores"]');
		expect(coresInput).not.toBeNull();
		expect(coresInput!.getAttribute('max')).toBe('8');

		coresInput!.value = '16';
		coresInput!.dispatchEvent(new Event('input', { bubbles: true }));
		flushSync();

		const saveBtn = document.querySelector<HTMLButtonElement>('[data-testid="vm-hardware-save"]');
		saveBtn!.click();
		flushSync();

		expect(updateSpy).not.toHaveBeenCalled();
		expect(coresInput!.getAttribute('aria-invalid')).toBe('true');
		expect(document.body.textContent).toContain('Enter a value between 1 and 8');
	});

	it('AddDiskDialog and ResizeDiskDialog wire maxDiskPerVMGB and disable full bus slots', async () => {
		storeInstance = new VmDetailStore('default', 101);
		storeInstance.entity = sampleEntity('stopped');
		storeInstance.hardwareOptions = sampleHardwareOptions();

		mount(AddDiskDialog, { target: document.body, props: { open: true } });
		flushSync();
		await tick();

		const addSizeInput = document.querySelector<HTMLInputElement>('[data-testid="add-disk-size"]');
		expect(addSizeInput).not.toBeNull();
		expect(addSizeInput!.getAttribute('max')).toBe('100');
		expect(document.body.textContent).toContain('1–100 GB allowed');

		const virtioOption = document.querySelector<HTMLOptionElement>('select option[value="virtio"]');
		expect(virtioOption).not.toBeNull();
		expect(virtioOption!.disabled).toBe(true);

		document.body.innerHTML = '';

		mount(ResizeDiskDialog, {
			target: document.body,
			props: { open: true, disk: storeInstance.entity.disks![0]! }
		});
		flushSync();
		await tick();

		const resizeSizeInput = document.querySelector<HTMLInputElement>('[data-testid="resize-disk-size"]');
		expect(resizeSizeInput).not.toBeNull();
		expect(resizeSizeInput!.getAttribute('max')).toBe('100');
		expect(document.body.textContent).toContain('21–100 GB allowed');
	});
});
