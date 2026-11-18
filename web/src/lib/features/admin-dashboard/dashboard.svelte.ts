import { get, put, ApiRequestError } from '#lib/shared/api/client.js';
import { setContext, getContext } from 'svelte';
import { STALE_REFRESH_MS } from '#lib/shared/visibility-refresh.js';
import { m } from '#lib/paraglide/messages.js';
import type { DashboardAlert } from './dashboard-alerts';
import type { OnboardingStep } from './onboarding';

export interface NodeSummary {
	clusterKey: string;
	cluster: string;
	name: string;
	status: string;
	vmCount: number;
	vmRunningCount: number;
	/** VMs carrying the PVMSS tag (adopted or not). */
	pvmssVMCount: number;
	/** RAM allocated to those VMs, in GB (the policy usage figure). */
	pvmssRamGB: number;
	/** Node capacity limits; 0 means none set. */
	policyMaxRamGB: number;
	policyMaxVMs: number;
	/** PVMSS VMs outside managed pools and not hidden on /admin/recovery. */
	unadoptedVMCount: number;
	cpuCores: number;
	cpuUsage: number;
	memoryTotalBytes: number;
	memoryUsedBytes: number;
	/** Absent when the catalog could not be read (readiness unknown). */
	state?: NodeState;
	/** catalog readiness codes, set when state is incomplete. */
	missing?: string[];
}

export type NodeState = 'offline' | 'incomplete' | 'disabled_with_vms' | 'ready';

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
	pvmssVMCount: number;
	pvmssVMStatusCounts: VMStatusCounts;
	otherVMCount: number;
	/** Reported nodes neither enabled nor hosting PVMSS VMs. */
	otherNodeCount: number;
	catalogUnavailable: boolean;
	storages: DashboardStorage[];
	recentChanges: DashboardChange[];
	version: string;
	refreshedAt: string;
	/** First-run setup steps; absent once every required step is done. */
	onboarding?: { steps: OnboardingStep[] };
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
	/** `Date.now()` of the last completed load - `refreshIfStale()` uses it to
	 * skip reloads fresher than the interval. */
	lastLoadedAt = 0;

	/** Reloads when the last completed load is older than `minIntervalMs` -
	 * the visibility/poll hook calls this instead of loading on every tick. */
	async refreshIfStale(minIntervalMs: number = STALE_REFRESH_MS): Promise<void> {
		if (Date.now() - this.lastLoadedAt <= minIntervalMs) return;
		await this.load();
	}

	/** Persists manual "done" marks for onboarding steps (a bypass), then
	 * applies the recomputed block without a full dashboard reload. */
	async completeOnboarding(steps: string[]): Promise<void> {
		const res = await put<{ onboarding: { steps: OnboardingStep[] } | null }>('/api/v1/admin/onboarding', { steps });
		if (this.summary) {
			const summary = { ...this.summary };
			if (res.onboarding) summary.onboarding = res.onboarding;
			else delete summary.onboarding;
			this.summary = summary;
		}
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		this.errorCode = null;

		try {
			this.summary = await get<DashboardSummary>('/api/v1/admin/dashboard');
			this.lastLoadedAt = Date.now();
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
