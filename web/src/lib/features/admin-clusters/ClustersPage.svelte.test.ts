import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import { setLocale } from '$lib/paraglide/runtime.js';
import ClustersPage from './ClustersPage.svelte';
import { AdminClustersStore, type AdminCluster } from './clusters.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const cluster: AdminCluster = {
	name: 'secondary',
	displayName: 'prod-pve',
	url: 'https://secondary.invalid',
	tlsInsecureSkipVerify: false,
	tokenId: 'pvmss@pve!service',
	tokenSet: true,
	removedAt: null,
	lastTestStatus: 'ok',
	lastTestAt: null,
	lastTestMessage: null,
	proxmoxVersion: '8.2.4',
	nodeCount: 2,
	vmCount: 18,
	snippetStorage: 'shared',
	cloudInitWriteEnabled: true
};

describe('ClustersPage remove confirmation', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});

	it('asks for confirmation before removing a cluster', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { status: 'ok' })));
		const store = new AdminClustersStore();
		store.clusters = [cluster];
		const removeSpy = vi.spyOn(store, 'remove');
		mount(ClustersPage, { target: document.body, props: { store } });
		await tick();

		const removeButton = document.querySelector<HTMLButtonElement>('button[aria-label="Remove secondary"]');
		expect(removeButton).not.toBeNull();
		removeButton!.click();
		await tick();

		// The confirmation dialog opened; nothing was deleted yet.
		expect(document.querySelector('[data-testid="cluster-remove-confirm"]')).not.toBeNull();
		expect(removeSpy).not.toHaveBeenCalled();

		const confirm = document.querySelector<HTMLButtonElement>('[data-testid="cluster-remove-confirm"]');
		confirm!.click();
		await tick();
		await tick();

		expect(removeSpy).toHaveBeenCalledWith('secondary');
		expect(store.clusters).toHaveLength(0);
	});

	it('closing the dialog leaves the cluster in place', async () => {
		const store = new AdminClustersStore();
		store.clusters = [cluster];
		const removeSpy = vi.spyOn(store, 'remove');
		mount(ClustersPage, { target: document.body, props: { store } });
		await tick();

		document.querySelector<HTMLButtonElement>('button[aria-label="Remove secondary"]')!.click();
		await tick();

		const cancel = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(
			(button) => button.textContent?.trim() === 'Cancel'
		);
		expect(cancel).toBeDefined();
		cancel!.click();
		await tick();

		expect(document.querySelector('[data-testid="cluster-remove-confirm"]')).toBeNull();
		expect(removeSpy).not.toHaveBeenCalled();
		expect(store.clusters).toHaveLength(1);
	});
});
