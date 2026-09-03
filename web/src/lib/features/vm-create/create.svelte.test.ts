import { afterEach, describe, it, expect, vi } from 'vitest';
import { ApiRequestError } from '$lib/shared/api/client';
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
			templates: [{ vmid: 9000, node: 'pve-node-01', name: 'tmpl', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' }],
			cloudInitTemplates: [],
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
		const err = new ApiRequestError(400, 'quota_exceeded', 'alice already owns 5 of 5 allowed VMs');
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

	it('falls back to a fixed message for no_snippet_storage', async () => {
		const err = new ApiRequestError(400, 'no_snippet_storage', 'no snippet-capable storage on the selected node: pve-node-01: enable the snippets content type on a storage of this node (no storage found)');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		expect(msg).not.toContain('pve-node-01');
	});

	it('falls back to the generic creation message for an unknown code', async () => {
		const err = new ApiRequestError(500, 'unknown_code', 'something else');
		const msg = await submitWithError(err);
		expect(msg).not.toBeNull();
		// Generic message — does not echo the raw server text.
		expect(msg).not.toContain('something else');
	});
});
