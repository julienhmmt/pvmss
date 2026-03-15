import { api } from '$lib/api/client';
import type { VM, VMAction } from '$lib/types/admin';

export function getAllVMs(): Promise<VM[]> {
	return api.get('/api/v1/admin/vms');
}

export function vmAction(vmid: number, action: VMAction): Promise<void> {
	return api.post(`/api/v1/admin/vms/${vmid}/action`, { action });
}
