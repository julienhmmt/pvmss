import { get, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface NodeSummary {
	name: string;
	status: string;
}

export interface StorageSummary {
	name: string;
	node: string;
	type: string;
	totalBytes: number;
	usedBytes: number;
}

export interface DashboardSummary {
	nodes: NodeSummary[];
	nodeCount: number;
	vmCount: number;
	storages: StorageSummary[];
	storageTotalBytes: number;
	storageUsedBytes: number;
	version: string;
	refreshedAt: string;
}

/**
 * DashboardStore manages the admin dashboard view. API responses are
 * $state.raw — they are API data, not form edits (constitution VII). One
 * store instance per admin dashboard page, via context.
 */
export class DashboardStore {
	summary = $state.raw<DashboardSummary | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.summary = await get<DashboardSummary>('/api/v1/admin/dashboard');
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.dashboard.loadError']();
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
