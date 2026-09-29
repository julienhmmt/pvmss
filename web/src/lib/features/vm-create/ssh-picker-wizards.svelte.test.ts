import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import StepBase from './_steps/StepBase.svelte';
import SimpleWizard from './SimpleWizard.svelte';
import { VmCreateStore, type VmCreateCatalog } from './create.svelte';
import { TaskTrayStore } from '$lib/features/tasks/tasks.svelte';
import { TaskOutcomeLedger } from '$lib/features/tasks/task-outcome-ledger.svelte';
import { ToastRegion } from '$lib/shared/ui/toast.svelte';
import type { ProfileSshKey } from '$lib/features/profile-ssh-keys/types';

let storeInstance: VmCreateStore;
const tray = new TaskTrayStore();
const toast = new ToastRegion();
const outcomeLedger = new TaskOutcomeLedger();

vi.mock('./create.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('./create.svelte')>();
	return { ...original, getVmCreateContext: () => storeInstance };
});

vi.mock('$lib/features/tasks/tasks.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('$lib/features/tasks/tasks.svelte')>();
	return { ...original, getTaskTrayContext: () => tray };
});

vi.mock('$lib/shared/ui/toast.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('$lib/shared/ui/toast.svelte')>();
	return { ...original, getToastContext: () => toast };
});

vi.mock('$lib/features/tasks/task-outcome-ledger.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('$lib/features/tasks/task-outcome-ledger.svelte')>();
	return { ...original, getTaskOutcomeLedgerContext: () => outcomeLedger };
});

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function catalogWithImages(): VmCreateCatalog {
	return {
		cluster: 'default',
		nodes: ['pve-node-01'],
		storages: [{ name: 'local-lvm', node: 'pve-node-01' }],
		bridges: [],
		isos: [],
		images: [{ storage: 'ceph-images', node: 'pve-node-01', file: 'debian-12.qcow2', sizeBytes: 2 * 1024 * 1024 * 1024 }],
		profiles: [],
		templates: [],
		cloudInitTemplates: [],
		cloudInitWriteEnabled: false,
		tags: []
	};
}

function profileKeys(): ProfileSshKey[] {
	return [1, 2].map((n) => ({
		id: `k${n}`,
		label: `key ${n}`,
		publicKey: `ssh-ed25519 BLOB${n} key-${n}`,
		fingerprint: `SHA256:k${n}`,
		createdAt: '2026-09-29T10:00:00Z'
	}));
}

afterEach(() => {
	document.body.innerHTML = '';
	vi.unstubAllGlobals();
});

describe('SshKeyPicker in the creation wizards', () => {
	it('renders the picker in the detailed wizard image mode', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys: profileKeys() })));
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWithImages();
		storeInstance.mode = 'detailed';
		storeInstance.sourceType = 'image';

		mount(StepBase, { target: document.body });

		expect(document.querySelector('[data-testid="ssh-key-picker"]')).not.toBeNull();
		await vi.waitFor(() => expect(document.querySelector('[data-testid="ssh-key-picker-option-k1"]')).not.toBeNull());
		expect(document.querySelector('[data-testid="ssh-key-picker-option-k2"]')).not.toBeNull();
		// <= 3 profile keys: all preselected by default (D8).
		expect(storeInstance.sshSelection.selectedIds).toEqual(['k1', 'k2']);
	});

	it('renders the picker in the simple wizard image mode', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys: profileKeys() })));
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWithImages();
		storeInstance.mode = 'simple';
		storeInstance.simpleSource = 'image';

		mount(SimpleWizard, { target: document.body });

		expect(document.querySelector('[data-testid="ssh-key-picker"]')).not.toBeNull();
		await vi.waitFor(() => expect(document.querySelector('[data-testid="ssh-key-picker-option-k1"]')).not.toBeNull());
	});

	it('degrades to the bare textarea when the profile cannot be loaded', () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
		storeInstance = new VmCreateStore();
		storeInstance.catalog = catalogWithImages();
		storeInstance.mode = 'simple';
		storeInstance.simpleSource = 'image';

		mount(SimpleWizard, { target: document.body });

		expect(document.querySelector('[data-testid="ssh-key-picker"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="ssh-key-picker-paste"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="ssh-key-picker-option-k1"]')).toBeNull();
	});
});
