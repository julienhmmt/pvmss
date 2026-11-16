import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import ProfilesPage from './ProfilesPage.svelte';
import type { AdminProfile } from './profiles.svelte';

const existing: AdminProfile = {
	id: 'small', label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi',
	enabled: true, description: ''
};

const expertJson = JSON.stringify({
	label: 'Expert', sockets: 2, cpuCores: 4, memoryMB: 8192, diskGB: 80, bus: 'virtio', description: 'note'
});

function renderPage() {
	const onCreate = vi.fn().mockResolvedValue(true);
	const onUpdate = vi.fn().mockResolvedValue(true);
	const noop = vi.fn();
	const component = mount(ProfilesPage, {
		target: document.body,
		props: {
			profiles: [existing],
			filteredProfiles: [existing],
			loading: false,
			error: null,
			saving: false,
			saveError: null,
			clusterOptions: [],
			cluster: 'default',
			onClusterChange: noop,
			search: '',
			busFilter: '',
			enabledFilter: 'all',
			busOptions: ['scsi', 'virtio'],
			sortBy: 'position',
			sortDir: 'asc',
			onSearchChange: noop,
			onBusFilterChange: noop,
			onEnabledFilterChange: noop,
			onSort: noop,
			onResetFilters: noop,
			selected: new Set<string>(),
			onToggleSelect: noop,
			onToggleSelectAll: noop,
			canReorder: false,
			onMove: noop,
			exportContent: () => '',
			onImport: vi.fn().mockResolvedValue([]),
			onDelete: noop,
			onToggle: noop,
			onClearSaveError: noop,
			onCreate,
			onUpdate
		}
	});
	return { component, onCreate, onUpdate };
}

function buttonByText(text: string): HTMLButtonElement {
	const button = [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === text);
	if (!button) throw new Error(`button "${text}" not rendered`);
	return button;
}

function editor(): HTMLTextAreaElement {
	const field = document.querySelector<HTMLTextAreaElement>('[data-testid="profile-expert-json"]');
	if (!field) throw new Error('expert JSON editor not rendered');
	return field;
}

async function openExpertEditor(): Promise<void> {
	buttonByText('New profile').click();
	await tick();
	buttonByText('Expert mode').click();
	await tick();
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('ProfilesPage expert mode', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('submits the values read from the JSON editor', async () => {
		const { component, onCreate } = renderPage();
		await openExpertEditor();

		const field = editor();
		field.value = expertJson;
		field.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		await tick();

		expect(onCreate).toHaveBeenCalledWith('Expert', 4, 8192, 80, 'virtio', 2, 'note');
		void unmount(component);
	});

	it('seeds the editor from the profile being edited and submits through onUpdate', async () => {
		const { component, onUpdate } = renderPage();
		buttonByText('Edit').click();
		await tick();
		buttonByText('Expert mode').click();
		await tick();

		const seeded = editor().value;
		expect(seeded).toContain('"label": "Small"');
		expect(seeded).toContain('"cpuCores": 1');
		expect(seeded).toContain('"bus": "scsi"');
		expect(seeded).not.toContain('"id"');

		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		await tick();

		expect(onUpdate).toHaveBeenCalledWith('small', 'Small', 1, 2048, 20, 'scsi', 1, '');
		void unmount(component);
	});

	it('shows a validation error and does not submit invalid JSON', async () => {
		const { component, onCreate } = renderPage();
		await openExpertEditor();

		const field = editor();
		field.value = '{ "label": "Oops" }';
		field.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();
		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();

		expect(document.querySelector('dialog')?.textContent).toMatch(/missing field/i);
		expect(onCreate).not.toHaveBeenCalled();
		void unmount(component);
	});

	it('returns to the form after a rejected submit, without parsing the JSON again', async () => {
		const { component } = renderPage();
		buttonByText('Edit').click();
		await tick();
		buttonByText('Expert mode').click();
		await tick();

		const field = editor();
		field.value = '{ "label": "Oops" }';
		field.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();
		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		buttonByText('Form mode').click();
		await tick();

		expect(document.querySelector('[data-testid="profile-expert-json"]')).toBeNull();
		expect(document.querySelector('dialog')?.textContent).not.toMatch(/missing field/i);
		expect(document.querySelector<HTMLInputElement>('dialog input[required]')?.value).toBe('Small');
		void unmount(component);
	});
});
