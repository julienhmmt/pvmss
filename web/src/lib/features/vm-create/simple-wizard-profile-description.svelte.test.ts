import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import SimpleWizard from './SimpleWizard.svelte';
import { VmCreateStore, type VmCreateCatalog } from './create.svelte';
import { TaskTrayStore } from '#lib/features/tasks/tasks.svelte.js';
import { TaskOutcomeLedger } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
import { ToastRegion } from '#lib/shared/ui/toast.svelte.js';
import { setLocale } from '#lib/paraglide/runtime.js';

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

function catalogWithProfiles(): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-01'],
		storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
		bridges: [],
		isos: [],
		profiles: [
			{
				id: 'small',
				label: 'Small',
				sockets: 1,
				cpuCores: 1,
				memoryMB: 2048,
				diskGB: 20,
				bus: 'scsi',
				description: 'Light **web** workloads',
				descriptionHtml: '<p>Light <strong>web</strong> workloads</p>'
			},
			{ id: 'medium', label: 'Medium', sockets: 1, cpuCores: 2, memoryMB: 4096, diskGB: 40, bus: 'scsi', description: '' }
		],
		templates: [],
		tags: []
	};
}

/** The RadioCard label that wraps the radio for one profile id. */
function cardFor(id: string): HTMLLabelElement {
	const radio = document.querySelector<HTMLInputElement>(`input[type="radio"][value="${id}"]`);
	const card = radio?.closest('label');
	if (!card) throw new Error(`no card for profile ${id}`);
	return card;
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('SimpleWizard profile descriptions', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('shows the server-rendered description as formatted elements under the spec lines', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWithProfiles();
		storeInstance.mode = 'simple';
		storeInstance.simpleSource = 'profile';

		mount(SimpleWizard, { target: document.body });

		const card = cardFor('small');
		const description = card.querySelector('[data-testid="profile-description"]');
		expect(description?.querySelector('strong')?.textContent).toBe('web');
		expect(description?.textContent?.trim()).toBe('Light web workloads');
		expect(card.textContent).not.toContain('**');
	});

	it('renders a profile without a description exactly as before', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWithProfiles();
		storeInstance.mode = 'simple';
		storeInstance.simpleSource = 'profile';

		mount(SimpleWizard, { target: document.body });

		const card = cardFor('medium');
		expect(card.querySelectorAll('span.block')).toHaveLength(2);
		expect(card.querySelector('[data-testid="profile-description"]')).toBeNull();
	});
});
