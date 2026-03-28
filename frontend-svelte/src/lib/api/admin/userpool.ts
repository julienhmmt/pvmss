import { api } from '$lib/api/client';
import type { Pool } from '$lib/types/admin';

export function getPools(): Promise<Pool[]> {
	return api.get('/api/v1/admin/userpool');
}

export function createPool(data: {
	pool_name: string;
	password: string;
}): Promise<void> {
	return api.post('/api/v1/admin/userpool', data);
}

export function deletePool(name: string): Promise<void> {
	return api.delete(`/api/v1/admin/userpool/${encodeURIComponent(name)}`);
}
