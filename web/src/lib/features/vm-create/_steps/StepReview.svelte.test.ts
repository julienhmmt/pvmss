import { describe, it, expect, vi, afterEach } from 'vitest';
import { flushSync, mount } from 'svelte';
import StepReview from './StepReview.svelte';
import { VmCreateStore, type VmCreateCatalog } from '../create.svelte';

vi.mock('../create.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('../create.svelte')>();
	return { ...original, getVmCreateContext: () => storeInstance };
});

// Assigned before mount in each test; the mock above hands it to StepReview.
let storeInstance: VmCreateStore;

const catalog: VmCreateCatalog = {
	cluster: 'default',
	nodes: ['pve-node-02'],
	storages: [{ name: 'ceph-data', node: 'pve-node-02' }],
	bridges: [{ name: 'vmbr2', node: 'pve-node-02' }],
	isos: [],
	images: [{ storage: 'local', node: 'pve-node-02', file: 'debian-12.qcow2', sizeBytes: 1 }],
	profiles: [{ id: 'small', label: 'Small', sockets: 1, cpuCores: 2, memoryMB: 2048, diskGB: 20, bus: 'scsi' }],
	templates: [],
	cloudInitTemplates: [],
	cloudInitWriteEnabled: false,
	tags: []
};

function mountDetailed(setup: (store: VmCreateStore) => void): void {
	storeInstance = new VmCreateStore();
	storeInstance.catalog = catalog;
	storeInstance.mode = 'detailed';
	storeInstance.name = 'web-03';
	storeInstance.node = 'pve-node-02';
	storeInstance.diskStorage = 'ceph-data';
	storeInstance.nics = [{ bridge: 'vmbr2', model: 'virtio' }];
	setup(storeInstance);
	mount(StepReview, { target: document.body });
}

function summary(): string {
	return document.querySelector('[data-testid="review-summary"]')?.textContent ?? '';
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('StepReview', () => {
	it('shows a readable summary first and the JSON request behind the switch', () => {
		mountDetailed((store) => {
			store.memoryMB = 4096;
		});
		expect(summary()).toContain('web-03');
		expect(summary()).toContain('vmbr2 · virtio');
		expect(summary()).toContain('4 GiB');
		expect(document.querySelector('[data-testid="review-request"]')).toBeNull();

		document.querySelector<HTMLButtonElement>('[role="switch"]')?.click();
		flushSync();
		const json = document.querySelector('[data-testid="review-request"]')?.textContent ?? '';
		expect(JSON.parse(json)).toMatchObject({ name: 'web-03', node: 'pve-node-02', network: [{ bridge: 'vmbr2' }] });
		expect(document.querySelector('[data-testid="review-summary"]')).toBeNull();
	});

	it('shows the profile, not the ignored explicit hardware, for an image with a profile', () => {
		mountDetailed((store) => {
			store.sourceType = 'image';
			store.imageFile = 'debian-12.qcow2';
			store.imageStorage = 'local';
			store.profileId = 'small';
			store.memoryMB = 8192;
		});
		expect(summary()).toContain('debian-12.qcow2');
		expect(summary()).toContain('Small');
		expect(summary()).not.toContain('8 GiB');
	});
});
