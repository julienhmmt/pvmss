import { describe, it, expect, vi } from 'vitest';
import { mount, flushSync } from 'svelte';
import StepBase from './StepBase.svelte';
import { VmCreateStore, type VmCreateCatalog } from '../create.svelte';

vi.mock('../create.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('../create.svelte')>();
	return { ...original, getVmCreateContext: () => storeInstance };
});

// Assigned before mount in each test; the mock above hands it to StepBase.
let storeInstance: VmCreateStore;

function catalogWith(templates: VmCreateCatalog['templates']): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-01'],
		storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
		bridges: [],
		isos: [],
		images: [],
		profiles: [],
		templates,
		cloudInitTemplates: [],
		tags: []
	};
}

function sourceOptions(): string[] {
	const firstSelect = document.querySelector('select');
	if (!firstSelect) return [];
	return [...firstSelect.querySelectorAll('option')].map((o) => o.value);
}

describe('StepBase source selector', () => {
	it('omits the template option when the catalog has no templates', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith([]);
		mount(StepBase, { target: document.body });
		expect(sourceOptions()).toEqual(['iso']);
		document.body.innerHTML = '';
	});

	it('offers both sources when at least one template is approved', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith([
			{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' }
		]);
		mount(StepBase, { target: document.body });
		expect(sourceOptions()).toEqual(['iso', 'template']);
		document.body.innerHTML = '';
	});

	it('explains the missing template source when no template is approved', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith([]);
		mount(StepBase, { target: document.body });
		expect(document.querySelector('[data-testid="no-templates-hint"]')).not.toBeNull();
		document.body.innerHTML = '';
	});

	it('hides the no-template hint once a template is approved', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith([
			{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' }
		]);
		mount(StepBase, { target: document.body });
		expect(document.querySelector('[data-testid="no-templates-hint"]')).toBeNull();
		document.body.innerHTML = '';
	});
});

describe('StepBase template disk minimum (issue 04)', () => {
	function mountWithTemplate(template: VmCreateCatalog['templates'][number], diskSizeGB: number): VmCreateStore {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith([template]);
		storeInstance.sourceType = 'template';
		storeInstance.diskSizeGB = diskSizeGB;
		mount(StepBase, { target: document.body });
		return storeInstance;
	}

	function templateSelect(): HTMLSelectElement {
		const selects = document.querySelectorAll('select');
		const last = selects[selects.length - 1];
		if (!last) throw new Error('template select not rendered');
		return last;
	}

	it('raises the disk size to the template minimum on selection', () => {
		const store = mountWithTemplate(
			{ vmid: 9000, node: 'pve-node-02', name: 'big', cloudInitCapable: false, diskSizeGB: 32, diskStorage: 'local-lvm' },
			20
		);
		const select = templateSelect();
		select.value = '9000';
		select.dispatchEvent(new Event('change', { bubbles: true }));
		expect(store.diskSizeGB).toBe(32);
		expect(store.templateMinDiskGB).toBe(32);
		document.body.innerHTML = '';
	});

	it('never auto-shrinks a larger user-set size', () => {
		const store = mountWithTemplate(
			{ vmid: 9001, node: 'pve-node-02', name: 'small', cloudInitCapable: false, diskSizeGB: 8, diskStorage: 'local' },
			40
		);
		const select = templateSelect();
		select.value = '9001';
		select.dispatchEvent(new Event('change', { bubbles: true }));
		expect(store.diskSizeGB).toBe(40);
		expect(store.templateMinDiskGB).toBe(8);
		document.body.innerHTML = '';
	});

	it('clears the minimum when the source switches back to ISO', () => {
		const store = mountWithTemplate(
			{ vmid: 9000, node: 'pve-node-02', name: 'big', cloudInitCapable: false, diskSizeGB: 32, diskStorage: 'local-lvm' },
			20
		);
		store.templateMinDiskGB = 32;
		store.sourceType = 'iso';
		flushSync();
		expect(store.templateMinDiskGB).toBe(0);
		document.body.innerHTML = '';
	});
});

describe('StepBase template picker grouping (issue 04)', () => {
	it('groups template options by node and carries size and cloud-init in the label', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWith([
			{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' },
			{ vmid: 9001, node: 'pve-node-01', name: 'alpine', cloudInitCapable: false, diskSizeGB: 2, diskStorage: 'local' },
			{ vmid: 9002, node: 'pve-node-02', name: 'rocky', cloudInitCapable: false, diskSizeGB: 4, diskStorage: 'local' }
		]);
		storeInstance.sourceType = 'template';
		mount(StepBase, { target: document.body });

		const groups = [...document.querySelectorAll('optgroup')];
		expect(groups.map((g) => g.label)).toEqual(['pve-node-01', 'pve-node-02']);
		const labels = groups.flatMap((g) => [...g.querySelectorAll('option')].map((o) => o.textContent ?? ''));
		expect(labels.some((l) => l.includes('debian-12-cloud') && l.includes('8 GB') && l.includes('cloud-init'))).toBe(true);
		document.body.innerHTML = '';
	});
});
