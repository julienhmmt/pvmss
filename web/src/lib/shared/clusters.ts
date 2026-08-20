import { get } from '$lib/shared/api/client';

export interface ClusterOption {
	name: string;
	displayName: string;
	oidcEnabled: boolean;
}

export async function fetchClusterOptions(): Promise<ClusterOption[]> {
	return get<ClusterOption[]>('/api/v1/auth/clusters');
}
