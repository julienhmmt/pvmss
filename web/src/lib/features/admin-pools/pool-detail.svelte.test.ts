import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PoolDetail } from './pool-detail.svelte';

const api = vi.hoisted(() => ({ get: vi.fn() }));
const quota = vi.hoisted(() => ({
	fetchQuotaOverrides: vi.fn(),
	setQuotaOverride: vi.fn(),
	removeQuotaOverride: vi.fn()
}));

vi.mock('#lib/shared/api/client.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/shared/api/client.js')>()),
	get: api.get
}));
vi.mock('#lib/features/quota-overrides/quota-overrides.js', () => quota);

import { PoolDetailStore } from './pool-detail.svelte';

const detail = {
	name: 'lab',
	username: 'lab@pve',
	comment: '',
	cluster: 'pve',
	managed: true,
	quota: { used: 1, allowed: 3 },
	vms: [],
	activity: []
} as unknown as PoolDetail;

const override = { cluster: 'pve', pool: 'lab', maxVms: 12, updatedBy: 'admin', updatedAt: '2026-11-01T00:00:00Z' };

describe('PoolDetailStore quota override', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	it('reads the override for this pool from the cluster list', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockResolvedValue([{ ...override, pool: 'other' }, override]);
		const store = new PoolDetailStore();

		await store.load('lab', 'pve');

		expect(quota.fetchQuotaOverrides).toHaveBeenCalledWith('pve');
		expect(store.override).toEqual(override);
		expect(store.overrideError).toBeNull();
	});

	it('keeps the detail page when the override list fails', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockRejectedValue(new Error('network down'));
		const store = new PoolDetailStore();

		await store.load('lab', 'pve');

		expect(store.detail).toEqual(detail);
		expect(store.error).toBeNull();
		expect(store.override).toBeNull();
		expect(store.overrideError).not.toBeNull();
	});

	it('saves the cap and reloads the detail so the allowance follows', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockResolvedValue([]);
		quota.setQuotaOverride.mockResolvedValue(override);
		const store = new PoolDetailStore();
		await store.load('lab', 'pve');

		await store.saveOverride(12);

		expect(quota.setQuotaOverride).toHaveBeenCalledWith('pve', 'lab', 12);
		expect(api.get).toHaveBeenCalledTimes(2);
	});

	it('removes the override and reloads', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockResolvedValue([override]);
		quota.removeQuotaOverride.mockResolvedValue(undefined);
		const store = new PoolDetailStore();
		await store.load('lab', 'pve');

		await store.removeOverride();

		expect(quota.removeQuotaOverride).toHaveBeenCalledWith('pve', 'lab');
		expect(api.get).toHaveBeenCalledTimes(2);
	});
});
