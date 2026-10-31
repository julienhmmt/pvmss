import { afterEach, describe, it, expect, vi } from 'vitest';
import { ApiRequestError } from '#lib/shared/api/client.js';
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

	it.each([true, false])('preserves the requested start status after an accepted creation: %s', async (startAfterCreate) => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ upid: 'task' }), {
			status: 202, headers: { 'Content-Type': 'application/json' }
		})));
		const store: VmCreateStore = new VmCreateStore();
		store.startAfterCreate = startAfterCreate;
		expect((await store.submit())?.expectedStatus).toBe(startAfterCreate ? 'running' : 'stopped');
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
			isos: [
				{ storage: 'local', node: 'pve-node-01', file: 'debian-12.iso' },
				{ storage: 'local', node: 'pve-node-02', file: 'ubuntu-24.04.iso' }
			],
			profiles: [
				{ id: 'small', label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi' }
			],
			templates: [
				{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', diskSizeGB: 8, diskStorage: 'local-lvm' }
			],
			tags: []
		};
	}

	it('builds a profile request with adjusted placement', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.profileId = 'small';
		store.nodeAdjusted = true;
		store.node = 'pve-node-01';
		store.storageAdjusted = true;
		store.storage = 'local-lvm';

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			profileId: 'small',
			node: 'pve-node-01',
			disk: { storage: 'local-lvm' },
			startAfterCreate: true,
			uefi: true
		});
	});

	it('never sends a secureBoot field - the EFI disk always gets an empty key store', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.profileId = 'small';

		const request = store.buildRequest();

		expect(request).not.toHaveProperty('secureBoot');
		expect(request.uefi).toBe(true);
	});

	it('builds a profile request with an ISO (auto node - server places on an ISO-holding node)', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.profileId = 'small';
		store.isoFile = 'ubuntu-24.04.iso';

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			profileId: 'small',
			iso: { storage: 'local', file: 'ubuntu-24.04.iso' },
			startAfterCreate: true,
			uefi: true
		});
	});

	it('builds a profile request with an ISO on the adjusted node', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.profileId = 'small';
		store.isoFile = 'debian-12.iso';
		store.nodeAdjusted = true;
		store.node = 'pve-node-01';

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			profileId: 'small',
			node: 'pve-node-01',
			iso: { storage: 'local', file: 'debian-12.iso' },
			startAfterCreate: true,
			uefi: true
		});
	});

	it('omits the ISO when the adjusted node does not hold it', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.profileId = 'small';
		store.isoFile = 'ubuntu-24.04.iso';
		store.nodeAdjusted = true;
		store.node = 'pve-node-01';

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			profileId: 'small',
			node: 'pve-node-01',
			startAfterCreate: true,
			uefi: true
		});
	});

	it('builds a template clone request without placement or disk', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.simpleSource = 'template';
		store.templateId = 9000;
		store.nodeAdjusted = true;
		store.storageAdjusted = true;

		expect(store.buildRequest()).toEqual({
			cluster: 'default',
			name: 'web-04',
			templateId: 9000,
			startAfterCreate: true
		});
	});
});

describe('VmCreateStore.submit error translation', () => {
	afterEach(() => vi.unstubAllGlobals());

	/** Stubs fetch to reject with the given ApiRequestError and submits. */
	async function submitWithError(err: ApiRequestError): Promise<string | null> {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(err));
		const store = new VmCreateStore();
		store.name = 'web-04';
		store.simpleSource = 'template';
		store.templateId = 9000;
		store.catalog = {
			cluster: 'musclegrid',
			nodes: ['pve-node-01'],
			storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
			bridges: [],
			isos: [],
			profiles: [],
			templates: [{ vmid: 9000, node: 'pve-node-01', name: 'tmpl', diskSizeGB: 8, diskStorage: 'local-lvm' }],
			tags: []
		};
		await store.submit();
		return store.submitError;
	}

	it('parses capacity_exceeded into a localized message naming the node and dimension', async () => {
		const err = new ApiRequestError(400, 'capacity_exceeded', 'node "miniquarium" disk capacity (48) would be exceeded');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		expect(msg).toContain('miniquarium');
		// The dimension label is localized (en: "disk", fr: "disque"); assert
		// the numeric capacity and that the raw English word was replaced.
		expect(msg).toContain('48');
		expect(msg).not.toContain('would be exceeded');
	});

	it('parses quota_exceeded into a localized message with used/allowed counts', async () => {
		const err = new ApiRequestError(400, 'quota_exceeded', 'pool "team-a" already holds 5 of 5 allowed VMs');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		expect(msg).toContain('5');
	});

	it('parses gabarit_exceeded (diskGB) into a localized message', async () => {
		const err = new ApiRequestError(400, 'gabarit_exceeded', 'disk size (64 GB) exceeds the configured gabarit (32 GB)');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		expect(msg).toContain('64');
		expect(msg).toContain('32');
	});

	it('parses out_of_range into a localized message with min/max', async () => {
		const err = new ApiRequestError(400, 'out_of_range', 'cpuCores must be between 1 and 32');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		expect(msg).toContain('1');
		expect(msg).toContain('32');
	});

	it('falls back to a fixed message for invalid_request', async () => {
		const err = new ApiRequestError(400, 'invalid_request', 'tpm requires uefi');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		// The fixed invalid_request message does not echo the raw server text.
		expect(msg).not.toContain('tpm requires uefi');
	});

	it('falls back to the generic creation message for an unknown code', async () => {
		const err = new ApiRequestError(500, 'unknown_code', 'something else');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		// Generic message - does not echo the raw server text.
		expect(msg).not.toContain('something else');
	});
});
