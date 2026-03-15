import { api } from '$lib/api/client';
import type { Storage } from '$lib/types/admin';

export function getStorages(): Promise<Storage[]> {
	return api.get('/api/v1/admin/storage');
}
