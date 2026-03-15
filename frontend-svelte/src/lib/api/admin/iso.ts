import { api } from '$lib/api/client';
import type { ISO } from '$lib/types/admin';

export function getISOs(): Promise<ISO[]> {
	return api.get('/api/v1/admin/iso');
}
