import { api } from '$lib/api/client';
import type { Limits } from '$lib/types/admin';

export function getLimits(): Promise<Limits> {
	return api.get('/api/v1/admin/limits');
}

export function updateLimits(limits: Limits): Promise<void> {
	return api.put('/api/v1/admin/limits', limits);
}
