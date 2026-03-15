import { api } from '$lib/api/client';
import type { VMBR } from '$lib/types/admin';

export function getVMBRs(): Promise<VMBR[]> {
	return api.get('/api/v1/admin/vmbr');
}
