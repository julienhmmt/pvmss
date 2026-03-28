import { api } from '$lib/api/client';
import type { ISO } from '$lib/types/admin';

export function getISOs(): Promise<ISO[]> {
	return api.get('/api/v1/admin/iso');
}

export function toggleISO(volid: string): Promise<void> {
	return api.post('/api/v1/admin/iso/toggle', { volid });
}
