import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import NodeCapacityImportDialog from './NodeCapacityImportDialog.svelte';

const exportJson = JSON.stringify({
	kind: 'pvmss-node-capacity',
	version: 1,
	cluster: 'default',
	nodes: [
		{ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 },
		{ node: 'pve-node-02', maxVms: 2, maxVcpus: 4, maxRamGb: 8, maxDiskGb: 50 }
	]
});

function render() {
	const onImport = vi.fn().mockResolvedValue([]);
	const onClose = vi.fn();
	const component = mount(NodeCapacityImportDialog, {
		target: document.body,
		props: { open: true, nodeNames: ['pve-node-01', 'pve2'], saving: false, saveError: null, onImport, onClose }
	});
	return { component, onImport, onClose };
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('NodeCapacityImportDialog', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('parses pasted JSON and submits every chosen node with the overwrite flag', async () => {
		const { component, onImport } = render();
		await tick();
		const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="node-capacity-import-json"]');
		expect(textarea).not.toBeNull();

		textarea!.value = exportJson;
		textarea!.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		expect(document.querySelector('dialog')?.textContent).toContain('pve-node-01');

		const overwrite = document.querySelector<HTMLInputElement>('[data-testid="node-import-overwrite"]');
		overwrite!.checked = true;
		overwrite!.dispatchEvent(new Event('change', { bubbles: true }));
		await tick();

		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		await tick();

		expect(onImport).toHaveBeenCalledWith(
			[
				{
					cluster: null,
					nodes: [
						{ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 },
						{ node: 'pve-node-02', maxVms: 2, maxVcpus: 4, maxRamGb: 8, maxDiskGb: 50 }
					]
				}
			],
			true
		);
		void unmount(component);
	});

	it('imports a node\'s caps onto another node', async () => {
		const { component, onImport } = render();
		await tick();
		const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="node-capacity-import-json"]')!;
		textarea.value = exportJson;
		textarea.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		expect(document.querySelector('[data-testid="node-import-target"]')).toBeNull();
		document.querySelector<HTMLInputElement>('[data-testid="node-import-retarget"]')!.click();
		await tick();

		const target = document.querySelector<HTMLSelectElement>('[data-testid="node-import-target"]')!;
		expect(target.value).toBe('pve-node-01');
		target.querySelector<HTMLOptionElement>('option[value="pve2"]')!.selected = true;
		target.dispatchEvent(new Event('change', { bubbles: true }));
		await tick();

		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		await tick();

		expect(onImport).toHaveBeenCalledWith([{ cluster: null, nodes: [{ node: 'pve2', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 }, { node: 'pve-node-02', maxVms: 2, maxVcpus: 4, maxRamGb: 8, maxDiskGb: 50 }] }], false);
		void unmount(component);
	});

	it('reports a file that is not a node capacity export', async () => {
		const { component, onImport } = render();
		await tick();
		const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="node-capacity-import-json"]');

		textarea!.value = JSON.stringify({ kind: 'pvmss-policy', version: 1, gabarit: {}, quota: {} });
		textarea!.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		expect(document.querySelector('dialog')?.textContent).toContain('not a PVMSS node capacity export');
		expect(onImport).not.toHaveBeenCalled();
		void unmount(component);
	});
});
