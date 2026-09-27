import { get, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';
import type { DashboardAlert } from './dashboard-alerts';

export interface NodeSummary {
	cluster: string;
	name: string;
	status: string;
	vmCount: number;
	vmRunningCount: number;
	cpuCores: number;
	cpuUsage: number;
	memoryTotalBytes: number;
	memoryUsedBytes: number;
}

export interface VMStatusCounts {
	running: number;
	paused: number;
	stopped: number;
	other: number;
}

export interface DashboardStorage {
	cluster: string;
	name: string;
	/** Empty for shared storage (Ceph, NFS, ...), listed once per cluster. */
	node?: string;
	type: string;
	shared: boolean;
	usedBytes: number;
	totalBytes: number;
	percent: number;
}

export interface DashboardChange {
	id: number;
	actor: string;
	cluster: string;
	vmid: number | null;
	action: string;
	timestamp: string;
	targetType: string;
	targetId: string;
}

export interface DashboardSummary {
	alerts: DashboardAlert[];
	nodes: NodeSummary[];
	nodeCount: number;
	vmCount: number;
	vmStatusCounts: VMStatusCounts;
	storages: DashboardStorage[];
	recentChanges: DashboardChange[];
	version: string;
	refreshedAt: string;
}

/**
 * DashboardStore manages the admin dashboard view. API responses are
 * $state.raw - they are API data, not form edits (constitution VII). One
 * store instance per admin dashboard page, via context.
 */
export class DashboardStore {
	summary = $state.raw<DashboardSummary | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	errorCode = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		this.errorCode = null;

		try {
			this.summary = await get<DashboardSummary>('/api/v1/admin/dashboard');
		} catch (err) {
			if (err instanceof ApiRequestError) {
				this.error = err.message;
				this.errorCode = err.code;
			} else {
				this.error = m['admin.dashboard.loadError']();
				this.errorCode = null;
			}
		} finally {
			this.loading = false;
		}
	}
}

const DASHBOARD_CONTEXT_KEY = Symbol('admin-dashboard');

export function setDashboardContext(): DashboardStore {
	const store = new DashboardStore();
	setContext(DASHBOARD_CONTEXT_KEY, store);
	return store;
}

export function getDashboardContext(): DashboardStore {
	return getContext<DashboardStore>(DASHBOARD_CONTEXT_KEY);
}
