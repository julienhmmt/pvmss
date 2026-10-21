import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import CloudInitTemplatesPage from './CloudInitTemplatesPage.svelte';
import type { AdminCloudInitTemplate } from './cloudInitTemplates.svelte';

const template: AdminCloudInitTemplate = {
	id: 'tpl-1',
	label: 'Base',
	content: '#cloud-config\n',
	enabled: true,
	document: { filename: 'pvmss-tpl-1-abc.yml', command: 'pvesm …', nodes: [] }
};

interface PageProps {
	templates: AdminCloudInitTemplate[];
	loading: boolean;
	error: string | null;
	saving: boolean;
	saveError: string | null;
	onVerify: () => void;
	clusterOptions: { name: string; displayName: string }[];
	cluster: string;
	onClusterChange: (value: string) => void;
	onCreate: (label: string, content: string) => Promise<boolean>;
	onUpdate: (id: string, label: string, content: string) => Promise<boolean>;
	onDelete: (id: string) => void;
	onToggle: (id: string, enabled: boolean) => void;
	onClearSaveError: () => void;
}

function baseProps(overrides: Partial<PageProps> = {}): PageProps {
	return {
		templates: [template],
		loading: false,
		error: null,
		saving: false,
		saveError: null,
		onVerify: vi.fn(),
		clusterOptions: [{ name: 'default', displayName: 'Default' }],
		cluster: 'default',
		onClusterChange: vi.fn(),
		onCreate: vi.fn().mockResolvedValue(true),
		onUpdate: vi.fn().mockResolvedValue(true),
		onDelete: vi.fn(),
		onToggle: vi.fn(),
		onClearSaveError: vi.fn(),
		...overrides
	};
}

function submitDialogForm(): void {
	const form = document.querySelector<HTMLFormElement>('dialog form');
	expect(form).not.toBeNull();
	form!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
}

describe('CloudInitTemplatesPage safety nets', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('asks for confirmation before deleting a template', async () => {
		const onDelete = vi.fn();
		mount(CloudInitTemplatesPage, { target: document.body, props: baseProps({ onDelete }) });
		await tick();

		const deleteButton = document.querySelector<HTMLButtonElement>('button[aria-label="Delete Base"]');
		expect(deleteButton).not.toBeNull();
		deleteButton!.click();
		await tick();

		expect(document.querySelector('[data-testid="cloudinit-delete-confirm"]')).not.toBeNull();
		expect(onDelete).not.toHaveBeenCalled();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-delete-confirm"]')!.click();
		await tick();
		expect(onDelete).toHaveBeenCalledWith('tpl-1');
	});

	it('keeps the form dialog open when the save fails', async () => {
		const onCreate = vi.fn().mockResolvedValue(false);
		mount(CloudInitTemplatesPage, { target: document.body, props: baseProps({ onCreate }) });
		await tick();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-new-template"]')!.click();
		await tick();
		expect(document.querySelector('dialog')).not.toBeNull();

		submitDialogForm();
		await tick();
		await tick();

		expect(onCreate).toHaveBeenCalled();
		// The draft is still there: the dialog did not close.
		expect(document.querySelector('dialog')).not.toBeNull();
	});

	it('closes the form dialog when the save succeeds', async () => {
		const onCreate = vi.fn().mockResolvedValue(true);
		mount(CloudInitTemplatesPage, { target: document.body, props: baseProps({ onCreate }) });
		await tick();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-new-template"]')!.click();
		await tick();
		submitDialogForm();
		await tick();
		await tick();

		expect(document.querySelector('dialog')).toBeNull();
	});

	it('renders the save error inside the dialog, not on the page behind it', async () => {
		mount(CloudInitTemplatesPage, { target: document.body, props: baseProps({ saveError: 'invalid YAML' }) });
		await tick();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-new-template"]')!.click();
		await tick();

		const dialog = document.querySelector('dialog');
		expect(dialog?.textContent).toContain('invalid YAML');
	});
});
