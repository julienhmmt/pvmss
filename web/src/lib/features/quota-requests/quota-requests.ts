import { get, post } from '#lib/shared/api/client.js';

/** A quota increase request as the server returns it. */
export interface QuotaRequest {
	id: number;
	requester: string;
	cluster: string;
	pool: string;
	currentQuota: number;
	requestedQuota: number;
	reason: string;
	status: 'pending' | 'approved' | 'denied';
	decidedBy: string;
	decidedAt: string;
	createdAt: string;
}

/** The requester's most recent request on their own pool, or null. */
export async function fetchMyQuotaRequest(): Promise<QuotaRequest | null> {
	const response = await get<{ request: QuotaRequest | null }>('/api/v1/quota-requests/mine');
	return response.request;
}

/** Files a request. The server takes the cluster and pool from the session. */
export function submitQuotaRequest(requestedQuota: number, reason: string): Promise<QuotaRequest> {
	return post<QuotaRequest>('/api/v1/quota-requests', { requestedQuota, reason });
}

/** Lists requests for the admin inbox. */
export async function fetchAdminQuotaRequests(status: 'pending' | 'approved' | 'denied' | 'all' = 'pending'): Promise<QuotaRequest[]> {
	const response = await get<{ requests: QuotaRequest[] }>(`/api/v1/admin/quota-requests?status=${status}`);
	return response.requests;
}

/** Approves a request. quota overrides the requested number when given. */
export function approveQuotaRequest(id: number, quota?: number): Promise<QuotaRequest> {
	return post<QuotaRequest>(`/api/v1/admin/quota-requests/${id}/approve`, quota === undefined ? {} : { quota });
}

/** Denies a request; the policy is left unchanged. */
export function denyQuotaRequest(id: number): Promise<QuotaRequest> {
	return post<QuotaRequest>(`/api/v1/admin/quota-requests/${id}/deny`);
}
