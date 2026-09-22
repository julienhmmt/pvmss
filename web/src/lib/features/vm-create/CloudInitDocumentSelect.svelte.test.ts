import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, flushSync } from 'svelte';
import CloudInitDocumentSelect from './CloudInitDocumentSelect.svelte';
import { VmCreateStore, type VmCreateCatalog } from './create.svelte';
import { m } from '$lib/paraglide/messages.js';

vi.mock('./create.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('./create.svelte')>();
	return { ...original, getVmCreateContext: () => storeInstance };
});

// Assigned before mount in each test; the mock above hands it to the
// component (same pattern as StepBase.svelte.test.ts).
let storeInstance: VmCreateStore;

function catalogWith(writeEnabled: boolean): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-01'],
		storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
		bridges: [],
		isos: [],
		images: [],
		profiles: [],
		templates: [],
		cloudInitTemplates: [{ id: 'web-server', label: 'Web server' }],
		cloudInitWriteEnabled: writeEnabled,
		tags: []
	};
}

describe('CloudInitDocumentSelect', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders admin templates and user files as two optgroups', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith(true);
		storeInstance.myCloudInitFiles = [{ id: 'dev-box', label: 'Dev box' }];
		mount(CloudInitDocumentSelect, { target: document.body });

		const groups = [...document.querySelectorAll('optgroup')].map((g) => g.label);
		expect(groups).toHaveLength(2);
		expect([...document.querySelectorAll('option')].map((o) => o.value)).toContain('t:web-server');
		expect([...document.querySelectorAll('option')].map((o) => o.value)).toContain('f:dev-box');
	});

	it('offers a selectable "none" option to clear a chosen document', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith(true);
		storeInstance.myCloudInitFiles = [{ id: 'dev-box', label: 'Dev box' }];
		mount(CloudInitDocumentSelect, { target: document.body });

		const none = [...document.querySelectorAll('option')].find((o) => o.value === '');
		expect(none).toBeDefined();
		expect(none!.disabled).toBe(false);
		expect(none!.textContent).toBe(m['vms.create.cloudinitNone']());
	});

	it('displays the encoded value of the store selection', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith(true);
		storeInstance.myCloudInitFiles = [{ id: 'dev-box', label: 'Dev box' }];
		storeInstance.cloudInitFileId = 'dev-box';
		mount(CloudInitDocumentSelect, { target: document.body });
		flushSync(); // the select binding's effect runs after the options render

		// The select's DOM value is the encoded 'f:<id>'; the decode direction
		// (select → store) is covered by the cloudInitDocumentValue round-trip
		// test in create.svelte.test.ts - jsdom's :checked does not reflect
		// option.selected, so a synthetic change event can't exercise it.
		expect(document.querySelector('select')!.value).toBe('f:dev-box');
	});

	it('renders the disabled hint instead of a select when the cluster has no snippet write target', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith(false);
		mount(CloudInitDocumentSelect, { target: document.body });

		expect(document.querySelector('select')).toBeNull();
		expect(document.body.textContent).toContain(m['vms.create.cloudinitDisabledHint']());
	});

	it('is visible for the image source (issue 04)', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith(true);
		storeInstance.simpleSource = 'image';
		mount(CloudInitDocumentSelect, { target: document.body });

		expect(document.querySelector('select')).not.toBeNull();
	});

	it('is hidden when there is nothing to offer', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = { ...catalogWith(true), cloudInitTemplates: [] };
		storeInstance.myCloudInitFiles = [];
		mount(CloudInitDocumentSelect, { target: document.body });

		expect(document.querySelector('select')).toBeNull();
	});
});
