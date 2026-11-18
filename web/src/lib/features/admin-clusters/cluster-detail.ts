// Admin cluster detail: the single-fetch payload and tab parsing. Pure
// helpers live here so the page stays a thin render layer and the logic is
// vitest-able.

import { get, post } from '#lib/shared/api/client.js';
import type { DashboardAlert } from '#lib/features/admin-dashboard/dashboard-alerts.js';
import type { DashboardStorage, NodeSummary, VMStatusCounts } from '#lib/features/admin-dashboard/dashboard.svelte.js';
import type { AdminCluster, ClusterTestResult } from './clusters.svelte.js';

export type ClusterTab = 'status' | 'infrastructure' | 'connection';

const CLUSTER_TABS: ReadonlySet<ClusterTab> = new Set(['infrastructure', 'connection']);

/** Maps the ?tab= query value to a tab; absent or unknown values land on status. */
export function parseClusterTab(raw: string | null): ClusterTab {
	if (raw === 'infrastructure' || raw === 'connection') return raw;
	return 'status';
}

/** Serializes a tab back to the query value; the default tab keeps URLs clean. */
export function clusterTabParam(tab: ClusterTab): string | null {
	return CLUSTER_TABS.has(tab) ? tab : null;
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

/** One persisted connection test, newest first in history listings. */
export interface ClusterTestEntry {
	testedAt: string;
	status: 'ok' | 'unreachable' | 'error';
	message?: string;
	proxmoxVersion?: string;
	/** Absent for rows backfilled from before history existed. */
	actor?: string;
	durationMs: number | null;
}

/** Page sizes the history filter offers; the server rejects any other. */
export const TEST_HISTORY_LIMITS = [5, 10, 25, 50, 100] as const;
export type TestHistoryLimit = (typeof TEST_HISTORY_LIMITS)[number];

/** Maps the ?tests= query value to a limit; absent or unknown values give 5. */
export function parseTestHistoryLimit(raw: string | null): TestHistoryLimit {
	const parsed = Number(raw);
	return TEST_HISTORY_LIMITS.find((limit) => limit === parsed) ?? 5;
}

export async function fetchClusterTests(name: string, limit: TestHistoryLimit): Promise<ClusterTestEntry[]> {
	const body = await get<{ tests: ClusterTestEntry[] }>(`/api/v1/admin/clusters/${encodeURIComponent(name)}/tests?limit=${limit}`);
	return body.tests;
}
