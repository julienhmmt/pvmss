// Admin cluster detail: the single-fetch payload and tab parsing. Pure
// helpers live here so the page stays a thin render layer and the logic is
// vitest-able.

import { get, post } from '#lib/shared/api/client.js';
import type { DashboardAlert } from '#lib/features/admin-dashboard/dashboard-alerts.js';
import type { DashboardStorage, NodeSummary, VMStatusCounts } from '#lib/features/admin-dashboard/dashboard.svelte.js';
import type { AdminCluster, ClusterTestResult } from './clusters.svelte.js';

export type ClusterTab = 'status' | 'infrastructure' | 'connection';

const CLUSTER_TABS: readonly ClusterTab[] = ['infrastructure', 'connection'];

/** Maps the ?tab= query value to a tab; absent or unknown values land on status. */
export function parseClusterTab(raw: string | null): ClusterTab {
	if (raw === 'infrastructure' || raw === 'connection') return raw;
	return 'status';
}

/** Serializes a tab back to the query value; the default tab keeps URLs clean. */
export function clusterTabParam(tab: ClusterTab): string | null {
	return CLUSTER_TABS.includes(tab) ? tab : null;
}

/** The cluster's slice of the dashboard, computed from its index only. */
export interface ClusterInventory {
	/** True when the index is missing or past its freshness window. */
	stale: boolean;
	refreshedAt?: string;
	/** Every indexed node, including nodes hosting no VM. */
	nodes: NodeSummary[];
	storages: DashboardStorage[];
	alerts: DashboardAlert[];
	vmCount: number;
	vmStatusCounts: VMStatusCounts;
}

/** GET /api/v1/admin/clusters/{name}: the cluster DTO plus its inventory. */
export interface ClusterDetail extends AdminCluster {
	inventory: ClusterInventory;
}

export async function fetchClusterDetail(name: string): Promise<ClusterDetail> {
	return get<ClusterDetail>(`/api/v1/admin/clusters/${encodeURIComponent(name)}`);
}

/** Runs the live connection test; the server stores the snapshot it gathered. */
export async function testClusterConnection(name: string): Promise<ClusterTestResult> {
	return post<ClusterTestResult>(`/api/v1/admin/clusters/${encodeURIComponent(name)}/test`);
}
