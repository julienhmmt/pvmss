import { afterEach, describe, it, expect, vi } from 'vitest';
import { VmCreateStore, type VmCreateCatalog } from './create.svelte';

describe('VmCreateStore.submit with a template disk minimum (issue 04)', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('blocks the request when the disk is below the template minimum', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = new VmCreateStore();
		store.mode = 'detailed';
		store.name = 'web-01';
		store.templateMinDiskGB = 32;
		store.diskSizeGB = 16;

		const result = await store.submit();

		expect(result).toBeNull();
		expect(fetchMock).not.toHaveBeenCalled();
		expect(store.submitError).not.toBeNull();
	});

	it('sends the request when the disk meets the template minimum', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(new Response(JSON.stringify({ task: 't-1' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
		);
		const store = new VmCreateStore();
		store.mode = 'detailed';
		store.name = 'web-01';
		store.templateMinDiskGB = 32;
		store.diskSizeGB = 32;

		const result = await store.submit();

		expect(result).not.toBeNull();
		expect(store.submitError).toBeNull();
	});
});

describe('VmCreateStore.buildRequest in simple mode', () => {
	function catalog(): VmCreateCatalog {
		return {
			cluster: 'default',
			nodes: ['pve-node-01'],
			storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
			bridges: [],
			isos: [],
			profiles: [
				{ id: 'small', label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi' }
			],
			templates: [
				{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' }
			],
			cloudInitTemplates: [],
			tags: []
		};
	}

	it('builds a profile request with optional cloud-init and adjusted placement', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.profileId = 'small';
		store.cloudInitTemplateId = 'ci-01';
		store.nodeAdjusted = true;
		store.node = 'pve-node-01';
		store.storageAdjusted = true;
		store.storage = 'local-lvm';

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			profileId: 'small',
			cloudInitTemplateId: 'ci-01',
			node: 'pve-node-01',
			disk: { storage: 'local-lvm' },
			startAfterCreate: true
		});
	});

	it('builds a template clone request without placement or disk', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.simpleSource = 'template';
		store.templateId = 9000;
		store.cloudInitTemplateId = 'ci-01';
		store.nodeAdjusted = true;
		store.storageAdjusted = true;

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			templateId: 9000,
			cloudInitTemplateId: 'ci-01',
			startAfterCreate: true
		});
	});
});
