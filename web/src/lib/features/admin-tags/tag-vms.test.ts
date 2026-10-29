import { beforeEach, describe, expect, it, vi } from 'vitest';

const getMock = vi.hoisted(() => vi.fn());
vi.mock('#lib/shared/api/client.js', () => ({ get: getMock }));

import { fetchTagVMs } from './tag-vms';

function vm(vmid: number, tags: string[] | null) {
	return { cluster: 'default', vmid, name: `vm-${vmid}`, node: 'pve1', status: 'running', tags };
}

describe('fetchTagVMs', () => {
	beforeEach(() => {
		getMock.mockReset();
	});

	it('keeps only VMs carrying the tag, across pages', async () => {
		getMock
			.mockResolvedValueOnce({ items: [vm(101, ['team-web']), vm(102, null)], total: 101 })
			.mockResolvedValueOnce({ items: [vm(201, ['team-web', 'pvmss'])], total: 101 });

		const result = await fetchTagVMs('default', 'team-web');

		expect(result.map((v) => v.vmid)).toEqual([101, 201]);
		expect(result[0]).toEqual({ cluster: 'default', vmid: 101, name: 'vm-101', node: 'pve1', status: 'running' });
		expect(getMock).toHaveBeenCalledTimes(2);
		expect(getMock.mock.calls[0]?.[0]).toBe('/api/v1/vms?cluster=default&scope=all&page=1&pageSize=100');
		expect(getMock.mock.calls[1]?.[0]).toContain('page=2');
	});

	it('returns an empty list when no VM carries the tag', async () => {
		getMock.mockResolvedValueOnce({ items: [vm(101, ['other'])], total: 1 });

		await expect(fetchTagVMs('default', 'team-web')).resolves.toEqual([]);
		expect(getMock).toHaveBeenCalledTimes(1);
	});

	it('stops on an empty page even when total says more', async () => {
		getMock.mockResolvedValueOnce({ items: [], total: 500 });

		await expect(fetchTagVMs('default', 'x')).resolves.toEqual([]);
		expect(getMock).toHaveBeenCalledTimes(1);
	});

	it('propagates API errors to the caller', async () => {
		getMock.mockRejectedValueOnce(new Error('boom'));

		await expect(fetchTagVMs('default', 'x')).rejects.toThrow('boom');
	});
});
