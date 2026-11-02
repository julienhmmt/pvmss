import { beforeEach, describe, expect, it, vi } from 'vitest';

const client = vi.hoisted(() => ({
	get: vi.fn(),
	put: vi.fn(),
	del: vi.fn()
}));

vi.mock('#lib/shared/api/client.js', () => client);

import { fetchQuotaOverrides, removeQuotaOverride, setQuotaOverride } from './quota-overrides';

describe('quota override API', () => {
	beforeEach(() => {
		client.get.mockReset();
		client.put.mockReset();
		client.del.mockReset();
	});

	it('lists overrides for one cluster and unwraps the response', async () => {
		const overrides = [{ cluster: 'pve', pool: 'lab', maxVms: 12, updatedBy: 'admin', updatedAt: '2026-11-01T00:00:00Z' }];
		client.get.mockResolvedValue({ overrides });

		await expect(fetchQuotaOverrides('pve')).resolves.toEqual(overrides);
		expect(client.get).toHaveBeenCalledWith('/api/v1/admin/quota-overrides?cluster=pve');
	});

	it('lists every cluster when no cluster is given', async () => {
		client.get.mockResolvedValue({ overrides: [] });

		await fetchQuotaOverrides();

		expect(client.get).toHaveBeenCalledWith('/api/v1/admin/quota-overrides');
	});

	it('puts the cap for one pool and encodes both path segments', async () => {
		client.put.mockResolvedValue({ pool: 'my lab' });

		await setQuotaOverride('pve 1', 'my lab', 7);

		expect(client.put).toHaveBeenCalledWith('/api/v1/admin/quota-overrides/pve%201/my%20lab', { maxVms: 7 });
	});

	it('deletes the override for one pool', async () => {
		client.del.mockResolvedValue(undefined);

		await removeQuotaOverride('pve', 'lab');

		expect(client.del).toHaveBeenCalledWith('/api/v1/admin/quota-overrides/pve/lab');
	});
});
