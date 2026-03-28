import { api } from './client';

export interface VMSummary {
	vmid: number;
	name: string;
	node: string;
	status: string;
	cpu: number;
	cpus: number;
	mem_mb: number;
	max_mem_mb: number;
	disk_mb: number;
	uptime: number;
	tags: string;
}

interface VMListResponse {
	vms: VMSummary[];
	total: number;
}

export async function getVMs(): Promise<VMSummary[]> {
	const res = await api.get<VMListResponse>('/api/v1/vms');
	return res.vms;
}
