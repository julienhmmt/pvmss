import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import PolicyImportDialog from './PolicyImportDialog.svelte';
import type { ClusterOption } from '#lib/shared/clusters.js';

const gabarit = {
	maxSockets: 4,
	maxCores: 8,
	maxMemoryMB: 16384,
	maxDiskPerVmGb: 500,
	maxNetworkCards: 4,
	maxSnapshots: 5,
	isolationVlanTag: 0,
	vmidMin: 0,
	vmidMax: 0,
	leaseDays: 0
};

const exportJson = JSON.stringify({ kind: 'pvmss-policy', version: 1, cluster: 'default', gabarit, quota: { maxVmPerUser: 3 } });

const clusterOptions: ClusterOption[] = [{ name: 'default', displayName: 'Default' }];

function render() {
	const onImport = vi.fn().mockResolvedValue([]);
	const onClose = vi.fn();
	const component = mount(PolicyImportDialog, {
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

describe('PolicyImportDialog', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('parses pasted JSON, targets the page cluster and submits the policy', async () => {
		const { component, onImport } = render();
		await tick();
		const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="policy-import-json"]');
		expect(textarea).not.toBeNull();

		textarea!.value = exportJson;
		textarea!.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		expect(document.querySelector('dialog')?.textContent).toContain('Target clusters');

		document.querySelector<HTMLFormElement>('dialog form')!.requestSubmit();
		await tick();
		await tick();

		expect(onImport).toHaveBeenCalledWith(['default'], { gabarit, quota: { maxVmPerUser: 3 } });
		void unmount(component);
	});

	it('reports a file that is not a policy export', async () => {
		const { component, onImport } = render();
		await tick();
		const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="policy-import-json"]');

		textarea!.value = JSON.stringify({ kind: 'pvmss-profiles', version: 1, profiles: [] });
		textarea!.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();

		expect(document.querySelector('dialog')?.textContent).toContain('not a PVMSS policy export');
		expect(document.querySelector('dialog')?.textContent).not.toContain('Target clusters');
		expect(onImport).not.toHaveBeenCalled();
		void unmount(component);
	});
});
