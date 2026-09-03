import { describe, it, expect, vi } from 'vitest';
import { mount } from 'svelte';
import StepDisk from './StepDisk.svelte';
import { VmCreateStore, type VmCreateCatalog } from '../create.svelte';

vi.mock('../create.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('../create.svelte')>();
	return { ...original, getVmCreateContext: () => storeInstance };
});

// Assigned before mount in each test; the mock above hands it to StepDisk.
let storeInstance: VmCreateStore;

function catalogWith(template: VmCreateCatalog['templates'][number]): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-02'],
		storages: [
			{ name: 'local-lvm', node: 'pve-node-02' },
			{ name: 'ceph-data', node: 'pve-node-02' }
		],
		bridges: [],
		isos: [],
		profiles: [],
		templates: [template],
		cloudInitTemplates: [],
		tags: []
	};
}

function mountWith(template: VmCreateCatalog['templates'][number], targetStorage: string): VmCreateStore {
	storeInstance = new VmCreateStore();
	storeInstance.catalog = catalogWith(template);
	storeInstance.node = 'pve-node-02';
	storeInstance.sourceType = 'template';
	storeInstance.templateId = template.vmid;
	storeInstance.diskStorage = targetStorage;
	mount(StepDisk, { target: document.body });
	return storeInstance;
}

function hint(): string | null {
	return document.querySelector('[data-testid="full-clone-hint"]')?.textContent ?? null;
}

describe('StepDisk full-clone hint (issue 04)', () => {
	it('shows the cloud-init hint even when the target storage matches', () => {
		mountWith(
			{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' },
			'local-lvm'
		);
		// The cloud-init variant mentions cloud-init in every locale; the
		// storage-differs variant never does.
		expect(hint()).toContain('cloud-init');
		document.body.innerHTML = '';
	});

	it('shows the storage-differs hint for a non-cloud-init template on another storage', () => {
		mountWith(
			{ vmid: 9001, node: 'pve-node-02', name: 'alpine', cloudInitCapable: false, diskSizeGB: 2, diskStorage: 'local-lvm' },
			'ceph-data'
		);
		expect(hint()).not.toBeNull();
		expect(hint()).not.toContain('cloud-init');
		document.body.innerHTML = '';
	});

	it('shows no hint for a non-cloud-init template on the same storage', () => {
		mountWith(
			{ vmid: 9001, node: 'pve-node-02', name: 'alpine', cloudInitCapable: false, diskSizeGB: 2, diskStorage: 'local-lvm' },
			'local-lvm'
		);
		expect(hint()).toBeNull();
		document.body.innerHTML = '';
	});
});
