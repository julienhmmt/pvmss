import { afterEach, describe, it, expect, vi } from 'vitest';
import { VmCreateStore } from './create.svelte';

describe('VmCreateStore.submit with a template disk minimum (issue 04)', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('blocks the request when the disk is below the template minimum', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = new VmCreateStore();
		store.mode = 'detailed';
		store.name = 'web-01';
		store.templateMinDiskGB = 32;
		store.diskSizeGB = 16;

		const result = await store.submit();

		expect(result).toBeNull();
		expect(fetchMock).not.toHaveBeenCalled();
		expect(store.submitError).not.toBeNull();
	});

	it('sends the request when the disk meets the template minimum', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(new Response(JSON.stringify({ task: 't-1' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
		);
		const store = new VmCreateStore();
		store.mode = 'detailed';
		store.name = 'web-01';
		store.templateMinDiskGB = 32;
		store.diskSizeGB = 32;

		const result = await store.submit();

		expect(result).not.toBeNull();
		expect(store.submitError).toBeNull();
	});
});
