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

function catalogWithoutProfiles(): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-01'],
		storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
		bridges: [],
		isos: [],
		images: [],
		profiles: [],
		templates: [],
		cloudInitTemplates: [],
		cloudInitWriteEnabled: false,
		tags: []
	};
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('SimpleWizard without VM profiles', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('explains that no size is approved instead of an empty size step', () => {
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWithoutProfiles();
		storeInstance.mode = 'simple';
		storeInstance.simpleSource = 'profile';

		mount(SimpleWizard, { target: document.body });

		expect(document.querySelector('[data-testid="vm-create-no-profiles"]')).not.toBeNull();
	});
});
