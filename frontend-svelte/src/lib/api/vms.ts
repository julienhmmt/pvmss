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

export interface VMSearchParams {
	q?: string;
	status?: string;
	node?: string;
}

export async function searchVMs(params: VMSearchParams): Promise<VMSummary[]> {
	const qs = new URLSearchParams();
	if (params.q) qs.set('q', params.q);
	if (params.status) qs.set('status', params.status);
	if (params.node) qs.set('node', params.node);
	const query = qs.toString();
	const res = await api.get<VMListResponse>(`/api/v1/vms${query ? '?' + query : ''}`);
	return res.vms;
}
