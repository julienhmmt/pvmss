import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import ProfilesPage from './ProfilesPage.svelte';
import type { AdminProfile } from './profiles.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

const existing: AdminProfile = {
	id: 'small', label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi',
	enabled: true, description: 'Light web workloads'
};

function renderPage(overrides: Partial<Record<string, unknown>> = {}) {
	const handlers = {
		onCreate: vi.fn().mockResolvedValue(true),
		onUpdate: vi.fn().mockResolvedValue(true)
	};
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
			busOptions: ['scsi'],
			sortBy: 'label',
			sortDir: 'asc',
			onSearchChange: noop,
			onBusFilterChange: noop,
			onEnabledFilterChange: noop,
			onSort: noop,
			onResetFilters: noop,
			onDelete: noop,
			onToggle: noop,
			onClearSaveError: noop,
			...handlers,
			...overrides
		}
	});
	return { component, handlers };
}

function descriptionField(): HTMLTextAreaElement {
	const field = document.querySelector<HTMLTextAreaElement>('dialog textarea');
	if (!field) throw new Error('description textarea not rendered');
	return field;
}

function setValue(field: HTMLInputElement | HTMLTextAreaElement, value: string): void {
	field.value = value;
	field.dispatchEvent(new Event('input', { bubbles: true }));
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('ProfilesPage description field', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('sends the typed description on create', async () => {
		const { component, handlers } = renderPage({ profiles: [], filteredProfiles: [] });
		const openButton = [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'New profile');
		openButton?.click();
		await tick();

		setValue(document.querySelector<HTMLInputElement>('dialog input[required]')!, 'Medium');
		setValue(descriptionField(), 'For web apps');
		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();

		expect(handlers.onCreate).toHaveBeenCalledWith('Medium', 1, 2048, 20, 'scsi', 1, 'For web apps');
		void unmount(component);
	});

	it('sends an emptied description on update', async () => {
		const { component, handlers } = renderPage();
		const editButton = [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'Edit');
		editButton?.click();
		await tick();

		expect(descriptionField().value).toBe('Light web workloads');
		setValue(descriptionField(), '');
		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();

		expect(handlers.onUpdate).toHaveBeenCalledWith('small', 'Small', 1, 2048, 20, 'scsi', 1, '');
		void unmount(component);
	});
});
