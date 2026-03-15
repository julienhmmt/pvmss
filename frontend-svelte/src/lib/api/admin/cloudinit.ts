import { api } from '$lib/api/client';
import type { CloudInitTemplate, SFTPStatus } from '$lib/types/admin';

interface CloudInitListResponse {
	templates: CloudInitTemplate[];
	sftp_status?: SFTPStatus;
}

export function getCloudInits(): Promise<CloudInitListResponse> {
	return api.get('/api/v1/admin/cloudinit');
}

export function createCloudInit(data: {
	name: string;
	description: string;
	storage: string;
	content: string;
}): Promise<void> {
	return api.post('/api/v1/admin/cloudinit', data);
}

export function updateCloudInit(
	id: string,
	data: { name: string; description: string; storage: string; content: string }
): Promise<void> {
	return api.put(`/api/v1/admin/cloudinit/${encodeURIComponent(id)}`, data);
}

export function deleteCloudInit(id: string): Promise<void> {
	return api.delete(`/api/v1/admin/cloudinit/${encodeURIComponent(id)}`);
}

export function toggleCloudInit(id: string): Promise<void> {
	return api.post(`/api/v1/admin/cloudinit/${encodeURIComponent(id)}/toggle`);
}
