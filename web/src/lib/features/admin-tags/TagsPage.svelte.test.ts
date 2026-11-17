import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import TagsPage from './TagsPage.svelte';
import type { AdminTag } from './tags.svelte';

const tag: AdminTag = { name: 'team-web', color: '#22c55e', vmCount: 5, protected: false };

interface PageProps {
	tags: AdminTag[];
	filteredTags: AdminTag[];
	loading: boolean;
	error: string | null;
	saving: boolean;
	saveError: string | null;
	clusterOptions: { name: string; displayName: string }[];
	cluster: string;
	onClusterChange: (value: string) => void;
	search: string;
	sortBy: 'name' | 'vmCount';
	sortDir: 'asc' | 'desc';
	onSearchChange: (value: string) => void;
	onSort: (column: 'name' | 'vmCount') => void;
	onResetFilters: () => void;
	onCreate: (name: string, color: string) => Promise<boolean>;
	onUpdateColor: (name: string, color: string) => Promise<boolean>;
	onDelete: (name: string) => Promise<boolean>;
	onClearSaveError: () => void;
}

function baseProps(overrides: Partial<PageProps> = {}): PageProps {
	return {
		tags: [tag],
		filteredTags: [tag],
		loading: false,
		error: null,
		saving: false,
		saveError: null,
		clusterOptions: [{ name: 'default', displayName: 'Default' }],
		cluster: 'default',
		onClusterChange: vi.fn(),
		search: '',
		sortBy: 'name',
		sortDir: 'asc',
		onSearchChange: vi.fn(),
		onSort: vi.fn(),
		onResetFilters: vi.fn(),
		onCreate: vi.fn().mockResolvedValue(true),
		onUpdateColor: vi.fn().mockResolvedValue(true),
		onDelete: vi.fn().mockResolvedValue(true),
		onClearSaveError: vi.fn(),
		...overrides
	};
}

function openCreateDialog(): void {
	const button = [...document.querySelectorAll<HTMLButtonElement>('button')].find(
		(candidate) => candidate.textContent?.trim() === 'New tag'
	);
	expect(button).toBeDefined();
	button!.click();
}

function submitDialogForm(): void {
	const form = document.querySelector<HTMLFormElement>('dialog form');
	const nameInput = form?.querySelector<HTMLInputElement>('input[type="text"]');
	if (nameInput && !nameInput.value) {
		nameInput.value = 'team-new';
		nameInput.dispatchEvent(new Event('input', { bubbles: true }));
	}
	expect(form).not.toBeNull();
	form!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
}

vi.mock('./tag-vms', () => ({ fetchTagVMs: vi.fn().mockResolvedValue([]) }));

const pvmss: AdminTag = { name: 'pvmss', color: '#4f46e5', vmCount: 2, protected: true };

function clickCard(name: string): void {
	document.querySelector<HTMLButtonElement>(`button[aria-label="Open tag ${name}"]`)!.click();
}

describe('TagsPage groups and drawer', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('splits protected tags into the System group', async () => {
		mount(TagsPage, { target: document.body, props: baseProps({ tags: [tag, pvmss], filteredTags: [tag, pvmss] }) });
		await tick();
		expect(document.querySelector('[data-testid="tags-group-system"]')?.textContent).toContain('pvmss');
		expect(document.querySelector('[data-testid="tags-group-custom"]')?.textContent).toContain('team-web');
		expect(document.querySelector('[data-testid="tags-group-custom"]')?.textContent).not.toContain('pvmss');
	});

	it('gates delete behind typing the tag name', async () => {
		const onDelete = vi.fn().mockResolvedValue(true);
		mount(TagsPage, { target: document.body, props: baseProps({ onDelete }) });
		await tick();
		clickCard('team-web');
		await tick();
		const zone = document.querySelector('[data-testid="tag-danger-zone"]')!;
		const button = zone.querySelector<HTMLButtonElement>('button')!;
		expect(button.disabled).toBe(true);
		const input = zone.querySelector<HTMLInputElement>('input')!;
		input.value = 'team-web';
		input.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();
		expect(button.disabled).toBe(false);
		button.click();
		await tick();
		expect(onDelete).toHaveBeenCalledWith('team-web');
	});

	it('shows the protected note instead of delete for pvmss', async () => {
		mount(TagsPage, { target: document.body, props: baseProps({ tags: [pvmss], filteredTags: [pvmss] }) });
		await tick();
		clickCard('pvmss');
		await tick();
		expect(document.querySelector('[data-testid="tag-protected-note"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="tag-danger-zone"]')).toBeNull();
	});

	it('saves a preset color for an existing tag', async () => {
		const onUpdateColor = vi.fn().mockResolvedValue(true);
		mount(TagsPage, { target: document.body, props: baseProps({ onUpdateColor }) });
		await tick();
		clickCard('team-web');
		await tick();
		document.querySelector<HTMLButtonElement>('button[aria-label="#ef4444"]')!.click();
		await tick();
		submitDialogForm();
		await tick();
		expect(onUpdateColor).toHaveBeenCalledWith('team-web', '#ef4444');
	});
});

describe('TagsPage create dialog', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('keeps the dialog open when the create fails', async () => {
		const onCreate = vi.fn().mockResolvedValue(false);
		mount(TagsPage, { target: document.body, props: baseProps({ onCreate }) });
		await tick();

		openCreateDialog();
		await tick();
		expect(document.querySelector('dialog')).not.toBeNull();

		submitDialogForm();
		await tick();
		await tick();

		expect(onCreate).toHaveBeenCalled();
		expect(document.querySelector('dialog')).not.toBeNull();
	});

	it('closes the dialog when the create succeeds', async () => {
		const onCreate = vi.fn().mockResolvedValue(true);
		mount(TagsPage, { target: document.body, props: baseProps({ onCreate }) });
		await tick();

		openCreateDialog();
		await tick();
		submitDialogForm();
		await tick();
		await tick();

		expect(document.querySelector('dialog')).toBeNull();
	});

	it('renders the save error inside the dialog', async () => {
		mount(TagsPage, { target: document.body, props: baseProps({ saveError: 'name already used' }) });
		await tick();

		openCreateDialog();
		await tick();

		expect(document.querySelector('dialog')?.textContent).toContain('name already used');
	});
});

describe('TagsPage toolbar', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	function resetButton(): HTMLButtonElement | undefined {
		return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
			(candidate) => candidate.textContent?.trim() === 'Reset filters'
		);
	}

	it('shows the result count and hides reset when filters are default', async () => {
		mount(TagsPage, { target: document.body, props: baseProps() });
		await tick();

		expect(document.querySelector('[data-testid="tags-count"]')?.textContent?.trim()).toBe('1 tags');
		expect(resetButton()).toBeUndefined();
	});

	it('shows the reset button when a filter is active', async () => {
		mount(TagsPage, { target: document.body, props: baseProps({ search: 'web' }) });
		await tick();

		expect(resetButton()).toBeDefined();
	});

	it('shows the reset button when sorting is non-default', async () => {
		mount(TagsPage, { target: document.body, props: baseProps({ sortBy: 'vmCount' }) });
		await tick();

		expect(resetButton()).toBeDefined();
	});
});
