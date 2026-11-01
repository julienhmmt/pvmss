import { beforeEach, describe, expect, it, vi } from 'vitest';

const client = vi.hoisted(() => ({
	get: vi.fn(),
	post: vi.fn()
}));

vi.mock('#lib/shared/api/client.js', () => client);

import {
	approveQuotaRequest,
	denyQuotaRequest,
	fetchAdminQuotaRequests,
	fetchMyQuotaRequest,
	submitQuotaRequest
} from './quota-requests';

describe('quota request API', () => {
	beforeEach(() => {
		client.get.mockReset();
		client.post.mockReset();
	});

	it('submits the requested number and reason only', async () => {
		client.post.mockResolvedValue({ id: 3 });

		await submitQuotaRequest(10, 'more for the lab');

		expect(client.post).toHaveBeenCalledWith('/api/v1/quota-requests', {
			requestedQuota: 10,
			reason: 'more for the lab'
		});
	});

	it('unwraps the mine response and returns null when there is none', async () => {
		client.get.mockResolvedValueOnce({ request: null });
		await expect(fetchMyQuotaRequest()).resolves.toBeNull();

		client.get.mockResolvedValueOnce({ request: { id: 4, status: 'pending' } });
		await expect(fetchMyQuotaRequest()).resolves.toEqual({ id: 4, status: 'pending' });
	});

	it('lists the pending inbox by default', async () => {
		client.get.mockResolvedValue({ requests: [] });

		await fetchAdminQuotaRequests();

		expect(client.get).toHaveBeenCalledWith('/api/v1/admin/quota-requests?status=pending');
	});

	it('sends an empty JSON body on approve unless an override is given', async () => {
		client.post.mockResolvedValue({ id: 5 });

		await approveQuotaRequest(5);
		expect(client.post).toHaveBeenLastCalledWith('/api/v1/admin/quota-requests/5/approve', {});

		await approveQuotaRequest(5, 6);
		expect(client.post).toHaveBeenLastCalledWith('/api/v1/admin/quota-requests/5/approve', { quota: 6 });
	});

	it('denies without a body', async () => {
		client.post.mockResolvedValue({ id: 7 });

		await denyQuotaRequest(7);

		expect(client.post).toHaveBeenCalledWith('/api/v1/admin/quota-requests/7/deny');
	});
});
