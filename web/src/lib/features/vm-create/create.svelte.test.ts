import { afterEach, describe, it, expect, vi } from 'vitest';
import { ApiRequestError } from '$lib/shared/api/client';
import { VmCreateStore, type VmCreateCatalog } from './create.svelte';

// The cloud-image message keys are not compiled into src/lib/paraglide yet
// (the translations land in web/messages/*.json in a follow-up). Add only the
// missing keys over the real module so the image-mode paths are testable;
// remove this mock once the keys exist in the message files.
vi.mock('$lib/paraglide/messages.js', async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	const messages = actual as { m: Record<string, (inputs?: Record<string, unknown>) => string> };
	return {
		...actual,
		m: {
			...messages.m,
			'vms.create.diskBelowImageMin': ({ min }: { min: number }) =>
				`Disk size is below the selected cloud image's minimum (${min} GB).`,
			'vms.create.errorDiskBelowImage': () => 'Disk size is below the selected cloud image.',
			'vms.create.errorImageRequired': () => 'A cloud image is required.',
			'vms.create.errorCiUserRequired': () => 'A username is required.',
			'vms.create.errorCiSshKeysRequired': () => 'At least one SSH public key is required.',
			'vms.create.errorCiIpRequired': () => 'An IP address is required in static mode.',
			'vms.create.errorCiUserDataPrefix': () => 'User-data must start with #cloud-config.',
			'vms.create.errorCiUserDataTooLarge': () => 'User-data exceeds the 16 KB limit.'
		}
	};
});

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
			isos: [
				{ storage: 'local', node: 'pve-node-01', file: 'debian-12.iso' },
				{ storage: 'local', node: 'pve-node-02', file: 'ubuntu-24.04.iso' }
			],
			images: [
				{ storage: 'ceph-images', node: 'pve-node-01', file: 'debian-12-generic.img', sizeBytes: 3 * 1024 * 1024 * 1024 },
				{ storage: 'ceph-images', node: 'pve-node-01', file: 'alpine-3.img', sizeBytes: Math.floor(2.5 * 1024 * 1024 * 1024) }
			],
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

	it('builds a profile request with an ISO (auto node — server places on an ISO-holding node)', () => {
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
			startAfterCreate: true
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
			startAfterCreate: true
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
			images: [],
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

describe('VmCreateStore cloud-image source (image mode)', () => {
	afterEach(() => vi.unstubAllGlobals());

	function catalog(): VmCreateCatalog {
		return {
			cluster: 'default',
			nodes: ['pve-node-01', 'pve-node-02'],
			storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
			bridges: [],
			isos: [{ storage: 'local', node: 'pve-node-01', file: 'debian-12.iso' }],
			images: [
				{ storage: 'ceph-images', node: 'pve-node-01', file: 'debian-12-generic.img', sizeBytes: 3 * 1024 * 1024 * 1024 },
				{ storage: 'ceph-images', node: 'pve-node-01', file: 'alpine-3.img', sizeBytes: Math.floor(2.5 * 1024 * 1024 * 1024) }
			],
			profiles: [],
			templates: [],
			cloudInitTemplates: [],
			tags: []
		};
	}

	/** Builds a store in detailed image mode with complete cloud-init. */
	function detailedImageStore(): VmCreateStore {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.mode = 'detailed';
		store.name = 'web-04';
		store.setSourceType('image');
		store.selectImage('ceph-images', 'debian-12-generic.img');
		store.ciUser = 'admin';
		store.ciSshKeysInput = 'ssh-ed25519 AAAA...';
		store.diskStorage = 'local-lvm';
		store.diskSizeGB = 8;
		return store;
	}

	it('selecting the cloud-image source clears the ISO and the template', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.isoFile = 'debian-12.iso';
		store.templateId = 9000;
		store.templateMinDiskGB = 8;
		store.startAfterCreate = false;

		store.setSourceType('image');

		expect(store.sourceType).toBe('image');
		expect(store.isoFile).toBe('');
		expect(store.templateId).toBe(0);
		expect(store.templateMinDiskGB).toBe(0);
		// Image mode defaults start-after-create to true: the VM is fully
		// configured at first boot.
		expect(store.startAfterCreate).toBe(true);
	});

	it('selecting the ISO source clears the cloud-image selection', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.setSourceType('image');
		store.selectImage('ceph-images', 'debian-12-generic.img');
		expect(store.imageFile).not.toBe('');

		store.setSourceType('iso');

		expect(store.sourceType).toBe('iso');
		expect(store.imageFile).toBe('');
		expect(store.imageStorage).toBe('');
		expect(store.imageMinDiskGB).toBe(0);
	});

	it('selecting the template source clears the cloud-image selection', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.setSourceType('image');
		store.selectImage('ceph-images', 'debian-12-generic.img');

		store.setSourceType('template');

		expect(store.sourceType).toBe('template');
		expect(store.imageFile).toBe('');
	});

	it('selecting the simple-mode cloud-image source clears the template and ISO', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.isoFile = 'debian-12.iso';
		store.templateId = 9000;

		store.setSimpleSource('image');

		expect(store.simpleSource).toBe('image');
		expect(store.isoFile).toBe('');
		expect(store.templateId).toBe(0);
		expect(store.startAfterCreate).toBe(true);
	});

	it('derives the disk floor from the image size, ceiled to GB', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();

		store.selectImage('ceph-images', 'debian-12-generic.img');
		expect(store.imageMinDiskGB).toBe(3);

		// 2.5 GB image → 3 GB floor (ceil).
		store.selectImage('ceph-images', 'alpine-3.img');
		expect(store.imageMinDiskGB).toBe(3);
	});

	it('blocks the request when the disk is below the image minimum', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = detailedImageStore();
		store.diskSizeGB = 2;

		const result = await store.submit();

		expect(result).toBeNull();
		expect(fetchMock).not.toHaveBeenCalled();
		expect(store.submitError).not.toBeNull();
	});

	it('sends the request when the disk covers the image minimum', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(new Response(JSON.stringify({ task: 't-1' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
		);
		const store = detailedImageStore();
		store.diskSizeGB = 3;

		const result = await store.submit();

		expect(result).not.toBeNull();
		expect(store.submitError).toBeNull();
	});

	it('blocks the request when cloud-init is incomplete (no SSH keys)', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = detailedImageStore();
		store.ciSshKeysInput = '';

		const result = await store.submit();

		expect(result).toBeNull();
		expect(fetchMock).not.toHaveBeenCalled();
		expect(store.submitError).not.toBeNull();
	});

	it('builds a detailed image request with cloudInit and no iso/templateId', () => {
		const store = detailedImageStore();
		store.ciIpMode = 'static';
		store.ciIpAddress = '192.168.1.50';
		store.ciGateway = '192.168.1.1';

		const request = store.buildRequest();

		expect(request.image).toEqual({
			storage: 'ceph-images',
			file: 'debian-12-generic.img',
			cloudInit: {
				user: 'admin',
				sshKeys: ['ssh-ed25519 AAAA...'],
				ipMode: 'static',
				ipAddress: '192.168.1.50',
				gateway: '192.168.1.1'
			}
		});
		expect(request.iso).toBeUndefined();
		expect(request.templateId).toBeUndefined();
	});

	it('omits static addressing and empty optional cloud-init fields in dhcp mode', () => {
		const store = detailedImageStore();

		const request = store.buildRequest();

		expect(request.image?.cloudInit).toEqual({
			user: 'admin',
			sshKeys: ['ssh-ed25519 AAAA...'],
			ipMode: 'dhcp'
		});
	});

	it('posts the image payload without iso or templateId', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(new Response(JSON.stringify({ task: 't-1' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
		);
		const store = detailedImageStore();

		await store.submit();

		const init = vi.mocked(fetch).mock.calls[0]?.[1] ?? {};
		const body = JSON.parse(String(init.body)) as Record<string, unknown>;
		expect(body.image).toBeDefined();
		expect(body.iso).toBeUndefined();
		expect(body.templateId).toBeUndefined();
	});

	it('builds a simple image request with the disk size and no iso/templateId', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.name = 'web-04';
		store.simpleSource = 'image';
		store.selectImage('ceph-images', 'debian-12-generic.img');
		store.ciUser = 'admin';
		store.ciSshKeysInput = 'ssh-ed25519 AAAA...';
		store.diskSizeGB = 8;

		const request = store.buildRequest();

		expect(request).toEqual({
			cluster: 'default',
			name: 'web-04',
			image: {
				storage: 'ceph-images',
				file: 'debian-12-generic.img',
				cloudInit: { user: 'admin', sshKeys: ['ssh-ed25519 AAAA...'], ipMode: 'dhcp' }
			},
			disk: { sizeGB: 8 },
			startAfterCreate: true
		});
		expect(request.iso).toBeUndefined();
		expect(request.templateId).toBeUndefined();
	});

	/** A catalog with one profile — the mandatory-profile path. */
	function catalogWithProfile(): VmCreateCatalog {
		return {
			...catalog(),
			profiles: [{ id: 'small', label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi' }]
		};
	}

	it('hasProfiles is false for an empty catalog and true once one exists', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		expect(store.hasProfiles()).toBe(false);

		store.catalog = catalogWithProfile();
		expect(store.hasProfiles()).toBe(true);
	});

	it('blocks image-mode submit until a profile is picked when the catalog has profiles', () => {
		const store = new VmCreateStore();
		store.catalog = catalogWithProfile();
		store.simpleSource = 'image';
		store.selectImage('ceph-images', 'debian-12-generic.img');
		store.ciUser = 'admin';
		store.ciSshKeysInput = 'ssh-ed25519 AAAA...';

		expect(store.imageModeBlocker()).not.toBeNull();

		store.profileId = 'small';
		expect(store.imageModeBlocker()).toBeNull();
	});

	it('does not require a profile in image mode when the catalog has none', () => {
		const store = new VmCreateStore();
		store.catalog = catalog();
		store.simpleSource = 'image';
		store.selectImage('ceph-images', 'debian-12-generic.img');
		store.ciUser = 'admin';
		store.ciSshKeysInput = 'ssh-ed25519 AAAA...';
		store.diskSizeGB = 8;

		expect(store.imageModeBlocker()).toBeNull();
	});

	it('sends profileId instead of disk for a simple-mode image request with a profile', () => {
		const store = new VmCreateStore();
		store.catalog = catalogWithProfile();
		store.name = 'web-04';
		store.simpleSource = 'image';
		store.selectImage('ceph-images', 'debian-12-generic.img');
		store.profileId = 'small';
		store.ciUser = 'admin';
		store.ciSshKeysInput = 'ssh-ed25519 AAAA...';

		const request = store.buildRequest();

		expect(request.profileId).toBe('small');
		expect(request.disk).toBeUndefined();
	});

	it('sends profileId alongside disk for a detailed-mode image request with a profile', () => {
		const store = detailedImageStore();
		store.catalog = catalogWithProfile();
		store.profileId = 'small';

		const request = store.buildRequest();

		expect(request.profileId).toBe('small');
		expect(request.image).toBeDefined();
	});
});
