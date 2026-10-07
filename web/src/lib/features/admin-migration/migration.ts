import { get, post } from '$lib/shared/api/client';

export type MigrationWarning = 'vms' | 'vcpus' | 'ram' | 'disk';
export type MigrationExclusionReason = 'offline' | 'not_allowed';

export interface MigrationCandidate {
	node: string;
	warnings: MigrationWarning[];
}

export interface MigrationExclusion {
	node: string;
	reason: MigrationExclusionReason;
	detail: string;
}

export interface MigrationPreflight {
	cluster: string;
	vmid: number;
	node: string;
	running: boolean;
	lock: string;
	blocked: boolean;
	blockers: string[];
	localDisks: string[];
	candidates: MigrationCandidate[];
	excluded: MigrationExclusion[];
}

export interface MigrationStart {
	cluster: string;
	vmid: number;
	upid: string;
	source: string;
	target: string;
}

function migratePath(cluster: string, vmid: number): string {
	return `/api/v1/admin/vms/${encodeURIComponent(cluster)}/${vmid}/migrate`;
}

export function fetchMigrationPreflight(cluster: string, vmid: number): Promise<MigrationPreflight> {
	return get<MigrationPreflight>(migratePath(cluster, vmid));
}

export function startMigration(cluster: string, vmid: number, target: string): Promise<MigrationStart> {
	return post<MigrationStart>(migratePath(cluster, vmid), { target });
}
