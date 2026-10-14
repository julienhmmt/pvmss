import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import { setLocale } from '$lib/paraglide/runtime.js';
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
	protectedFilter: 'all' | 'protected' | 'unprotected';
	sortBy: 'name' | 'vmCount';
	sortDir: 'asc' | 'desc';
	onSearchChange: (value: string) => void;
	onProtectedFilterChange: (value: 'all' | 'protected' | 'unprotected') => void;
	onSort: (column: 'name' | 'vmCount') => void;
	onResetFilters: () => void;
	onCreate: (name: string, color: string) => Promise<boolean>;
	onUpdateColor: (name: string, color: string) => void;
	onDelete: (name: string) => void;
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
		protectedFilter: 'all',
		sortBy: 'name',
		sortDir: 'asc',
		onSearchChange: vi.fn(),
		onProtectedFilterChange: vi.fn(),
		onSort: vi.fn(),
		onResetFilters: vi.fn(),
		onCreate: vi.fn().mockResolvedValue(true),
		onUpdateColor: vi.fn(),
		onDelete: vi.fn(),
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
	const form = document.querySelector<HTMLFormElement>('[role="dialog"] form');
	expect(form).not.toBeNull();
	form!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
}

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
		expect(document.querySelector('[role="dialog"]')).not.toBeNull();

		submitDialogForm();
		await tick();
		await tick();

		expect(onCreate).toHaveBeenCalled();
		expect(document.querySelector('[role="dialog"]')).not.toBeNull();
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

		expect(document.querySelector('[role="dialog"]')).toBeNull();
	});

	it('renders the save error inside the dialog', async () => {
		mount(TagsPage, { target: document.body, props: baseProps({ saveError: 'name already used' }) });
		await tick();

		openCreateDialog();
		await tick();

		expect(document.querySelector('[role="dialog"]')?.textContent).toContain('name already used');
	});
});
