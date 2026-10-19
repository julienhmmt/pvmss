import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import SimpleWizard from './SimpleWizard.svelte';
import DetailedWizard from './DetailedWizard.svelte';
import { VmCreateStore, type VmCreateCatalog } from './create.svelte';
import { TaskTrayStore } from '#lib/features/tasks/tasks.svelte.js';
import { TaskOutcomeLedger } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
import { ToastRegion } from '#lib/shared/ui/toast.svelte.js';

let storeInstance: VmCreateStore;
const tray = new TaskTrayStore();
const toast = new ToastRegion();
const outcomeLedger = new TaskOutcomeLedger();

vi.mock('./create.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('./create.svelte')>();
	return { ...original, getVmCreateContext: () => storeInstance };
});

vi.mock('#lib/features/tasks/tasks.svelte.js', async (importOriginal) => {
	const original = await importOriginal<typeof import('#lib/features/tasks/tasks.svelte.js')>();
	return { ...original, getTaskTrayContext: () => tray };
});

vi.mock('#lib/shared/ui/toast.svelte.js', async (importOriginal) => {
	const original = await importOriginal<typeof import('#lib/shared/ui/toast.svelte.js')>();
	return { ...original, getToastContext: () => toast };
});

vi.mock('#lib/features/tasks/task-outcome-ledger.svelte.js', async (importOriginal) => {
	const original = await importOriginal<typeof import('#lib/features/tasks/task-outcome-ledger.svelte.js')>();
	return { ...original, getTaskOutcomeLedgerContext: () => outcomeLedger };
});

function baseCatalog(): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-01'],
		storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
		bridges: [{ name: 'vmbr0', node: 'pve-node-01' }],
		isos: [],
		images: [{ storage: 'ceph-images', node: 'pve-node-01', file: 'debian-12.qcow2', sizeBytes: 2 * 1024 * 1024 * 1024 }],
		profiles: [{ id: 'small', label: 'Small', sockets: 1, cpuCores: 2, memoryMB: 2048, diskGB: 20, bus: 'scsi' }],
		templates: [{ vmid: 9000, node: 'pve-node-01', name: 'debian-12-tpl', cloudInitCapable: true, diskSizeGB: 8, diskStorage: 'local-lvm' }],
		cloudInitTemplates: [],
		cloudInitWriteEnabled: false,
		tags: []
	};
}

describe('Create VM wizard validation timing (Ticket 09)', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	afterEach(() => {
		document.body.innerHTML = '';
		vi.unstubAllGlobals();
	});

	it('SimpleWizard shows no validation errors on initial mount before user interaction', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = baseCatalog();
		storeInstance.mode = 'simple';

		mount(SimpleWizard, { target: document.body });

		expect(document.querySelectorAll('[role="alert"]')).toHaveLength(0);
		expect(document.querySelectorAll('[aria-invalid="true"]')).toHaveLength(0);
	});

	it('SimpleWizard reveals errors and focuses the first invalid control on submit', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		storeInstance = new VmCreateStore();
		storeInstance.catalog = baseCatalog();
		storeInstance.mode = 'simple';
		storeInstance.profileId = 'small';

		mount(SimpleWizard, { target: document.body });

		const submitBtn = document.querySelector<HTMLButtonElement>('[data-testid="vm-create-submit"]');
		expect(submitBtn).not.toBeNull();
		expect(submitBtn!.disabled).toBe(false);

		submitBtn!.click();
		flushSync();
		await tick();

		expect(fetchMock).not.toHaveBeenCalled();
		expect(document.body.textContent).toContain('Name is required.');
		const nameInput = document.querySelector<HTMLInputElement>('input[placeholder="web-04"]');
		expect(nameInput).not.toBeNull();
		expect(nameInput!.getAttribute('aria-invalid')).toBe('true');
		expect(document.activeElement).toBe(nameInput);
	});

	it('DetailedWizard blocks Next on Base step when required fields are missing and focuses the first invalid field', async () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = baseCatalog();
		storeInstance.mode = 'detailed';

		mount(DetailedWizard, { target: document.body });

		expect(document.querySelectorAll('[role="alert"]')).toHaveLength(0);

		const nextBtn = [...document.querySelectorAll<HTMLButtonElement>('button')].find(
			(btn) => btn.textContent?.trim() === 'Next'
		);
		expect(nextBtn).toBeDefined();
		nextBtn!.click();
		flushSync();
		await tick();

		const baseTab = document.querySelector<HTMLButtonElement>('[role="tab"][aria-current="step"]');
		expect(baseTab?.textContent).toContain('Base');
		expect(document.body.textContent).toContain('Name is required.');
		expect(document.body.textContent).toContain('Select an approved node.');
		const nameInput = document.querySelector<HTMLInputElement>('input[placeholder="web-03"]');
		expect(document.activeElement).toBe(nameInput);

		storeInstance.name = 'web-valid';
		storeInstance.node = 'pve-node-01';
		flushSync();

		nextBtn!.click();
		flushSync();
		await tick();

		const currentTab = document.querySelector<HTMLButtonElement>('[role="tab"][aria-current="step"]');
		expect(currentTab?.textContent).toContain('Hardware');
	});
});
