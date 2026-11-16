import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import ProfileImportDialog from './ProfileImportDialog.svelte';
import type { ClusterOption } from '#lib/shared/clusters.js';

const exportJson = JSON.stringify({
	kind: 'pvmss-profiles',
	version: 1,
	profiles: [
		{ label: 'Small', sockets: 1, cpuCores: 1, memoryMB: 2048, diskGB: 20, bus: 'scsi', enabled: true, description: '' }
	]
});

const clusterOptions: ClusterOption[] = [{ name: 'default', displayName: 'Default' }];

function render() {
	const onImport = vi.fn().mockResolvedValue([]);
	const onClose = vi.fn();
	const component = mount(ProfileImportDialog, {
		target: document.body,
		props: {
			open: true,
			clusterOptions,
			cluster: 'default',
			saving: false,
			saveError: null,
			onImport,
			onClose
		}
	});
	return { component, onImport, onClose };
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('ProfileImportDialog', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('uses a styled file picker with a visible chosen-file label', () => {
		const { component } = render();
		const file = document.querySelector<HTMLInputElement>('[data-testid="profile-import-file"]');
		expect(file).not.toBeNull();
		expect(file!.className).toContain('sr-only');
		expect(file!.closest('label')?.textContent).toContain('Choose file');
		expect(document.querySelector('dialog')?.textContent).toContain('No file chosen');
		void unmount(component);
	});

	it('parses pasted JSON and shows the rest of the form', async () => {
		const { component, onImport } = render();
		await tick();
		const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="profile-import-json"]');
		expect(textarea).not.toBeNull();

		textarea!.value = exportJson;
		textarea!.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		expect(document.querySelector('dialog')?.textContent).toContain('Small');
		expect(document.querySelector('dialog')?.textContent).toContain('Target clusters');

		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		await tick();

		expect(onImport).toHaveBeenCalledWith(['default'], false, [
			expect.objectContaining({ label: 'Small' })
		]);
		void unmount(component);
	});
});
