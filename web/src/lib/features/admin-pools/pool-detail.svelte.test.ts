import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PoolDetail } from './pool-detail.svelte';

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }));
const quota = vi.hoisted(() => ({
	fetchQuotaOverrides: vi.fn(),
	setQuotaOverride: vi.fn(),
	removeQuotaOverride: vi.fn()
}));

vi.mock('#lib/shared/api/client.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/shared/api/client.js')>()),
	get: api.get,
	put: api.put
}));
vi.mock('#lib/features/quota-overrides/quota-overrides.js', () => quota);

import { PoolDetailStore, parsePoolTab, poolTabParam } from './pool-detail.svelte';

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

describe('PoolDetailStore resetPassword', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	it('asks the server to generate when no password is supplied', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockResolvedValue([]);
		api.put.mockResolvedValue({ username: 'lab@pve', password: 'generated-1' });
		const store = new PoolDetailStore();
		await store.load('lab', 'pve');

		const credentials = await store.resetPassword('');

		expect(api.put).toHaveBeenCalledWith('/api/v1/admin/pools/lab/password?cluster=pve', { password: '' });
		expect(credentials).toEqual({ username: 'lab@pve', password: 'generated-1' });
		expect(store.resetError).toBeNull();
	});

	it('sends the admin-supplied password verbatim', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockResolvedValue([]);
		api.put.mockResolvedValue({ username: 'lab@pve', password: 'custom-1' });
		const store = new PoolDetailStore();
		await store.load('lab', 'pve');

		await store.resetPassword('custom-1');

		expect(api.put).toHaveBeenCalledWith('/api/v1/admin/pools/lab/password?cluster=pve', { password: 'custom-1' });
	});

	it('surfaces the API failure and rethrows so the dialog stays open', async () => {
		api.get.mockResolvedValue(detail);
		quota.fetchQuotaOverrides.mockResolvedValue([]);
		const { ApiRequestError } = await import('#lib/shared/api/client.js');
		const failure = new ApiRequestError(502, 'reset_failed', 'pool password reset failed');
		api.put.mockRejectedValue(failure);
		const store = new PoolDetailStore();
		await store.load('lab', 'pve');

		await expect(store.resetPassword('')).rejects.toThrow(failure);

		expect(store.resetError).toBe(failure.message);
		expect(store.resetting).toBe(false);
	});
});

describe('parsePoolTab', () => {
	it.each([
		['overview', 'overview'],
		['members', 'members'],
		['activity', 'activity'],
		['bogus', 'overview'],
		['', 'overview'],
		[null, 'overview']
	] as const)('maps %s to %s', (raw, want) => {
		expect(parsePoolTab(raw)).toBe(want);
	});
});

describe('poolTabParam', () => {
	it.each([
		['overview', null],
		['members', 'members'],
		['activity', 'activity']
	] as const)('serializes %s to %s', (tab, want) => {
		expect(poolTabParam(tab)).toBe(want);
	});
});
