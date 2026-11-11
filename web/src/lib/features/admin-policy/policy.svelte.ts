import { get, post, put, ApiRequestError } from '#lib/shared/api/client.js';
import {
	fetchClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from '#lib/shared/clusters.js';
import { getContext, setContext } from 'svelte';
import { m } from '#lib/paraglide/messages.js';
import { fetchQuotaOverrides, type QuotaOverride } from '#lib/features/quota-overrides/quota-overrides.js';

export interface Gabarit {
	maxSockets: number;
	maxCores: number;
	maxMemoryMB: number;
	maxDiskPerVmGb: number;
	maxNetworkCards: number;
	maxSnapshots: number;
	isolationVlanTag: number;
	vmidMin: number;
	vmidMax: number;
	leaseDays: number;
}

export interface Quota {
	maxVmPerUser: number;
}

export interface AdminPolicy {
	cluster: string;
	gabarit: Gabarit;
	quota: Quota;
}

export interface GabaritPatch {
	maxSockets?: number;
	maxCores?: number;
	maxMemoryMB?: number;
	maxDiskPerVmGb?: number;
	maxNetworkCards?: number;
	maxSnapshots?: number;
	isolationVlanTag?: number;
	vmidMin?: number;
	vmidMax?: number;
	leaseDays?: number;
}

export interface AdminPolicyPatch {
	gabarit?: GabaritPatch;
	quota?: Partial<Quota>;
}

/** Manages server-owned global gabarit, quota, and YAML policy data. */
export class AdminPolicyStore {
	policy = $state.raw<AdminPolicy | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	errorCode = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	saved = $state.raw(false);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('');
	/** Per-pool caps on the selected cluster; they sit beside the global quota. */
	overrides = $state.raw<QuotaOverride[]>([]);
	overridesError = $state.raw<string | null>(null);
	/** Latest-wins guard for load: a cluster switch must not show the previous cluster's policy. */
	#loadRequestId = 0;

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			if (this.clusterOptions.length > 0) {
				this.cluster = resolveInitialAdminCluster(this.clusterOptions, this.cluster);
			}
		} catch (error: unknown) {
			this.errorCode = error instanceof ApiRequestError ? error.code : null;
			this.error = error instanceof ApiRequestError ? error.message : m['policy.loadError']();
		}
	}

	setCluster(value: string): void {
		saveAdminCluster(value);
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		const requestId = ++this.#loadRequestId;
		await this.loadClusters();
		if (requestId !== this.#loadRequestId) return;
		this.loading = true;
		this.error = null;
		this.errorCode = null;
		try {
			const policy = await get<AdminPolicy>(`/api/v1/admin/policy?cluster=${encodeURIComponent(this.cluster)}`);
			if (requestId === this.#loadRequestId) this.policy = policy;
		} catch (error: unknown) {
			if (requestId === this.#loadRequestId) {
				this.errorCode = error instanceof ApiRequestError ? error.code : null;
				this.error = error instanceof ApiRequestError ? error.message : m['policy.loadError']();
			}
		} finally {
			if (requestId === this.#loadRequestId) this.loading = false;
		}
		if (requestId === this.#loadRequestId && this.error === null) await this.loadOverrides();
	}

	/** Reads the cluster's overrides; a failure is shown beside the policy, not instead of it. */
	async loadOverrides(): Promise<void> {
		if (this.cluster === '') {
			this.overrides = [];
			return;
		}
		try {
			this.overrides = await fetchQuotaOverrides(this.cluster);
			this.overridesError = null;
		} catch (error: unknown) {
			this.overrides = [];
			this.overridesError = error instanceof ApiRequestError ? error.message : m['policy.overrides.loadError']();
		}
	}

	async save(patch: AdminPolicyPatch): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.saved = false;
		try {
			this.policy = await put<AdminPolicy>('/api/v1/admin/policy', { cluster: this.cluster, ...patch });
			this.saved = true;
		} catch (error: unknown) {
			this.saveError = error instanceof ApiRequestError ? error.message : m['policy.saveError']();
			throw error;
		} finally {
			this.saving = false;
		}
	}

	async retryConnection(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// Ignore; the next load will surface the current state.
		}
		await this.load();
	}
}

const POLICY_CONTEXT_KEY = Symbol('admin-policy');

export function setAdminPolicyContext(): AdminPolicyStore {
	const store = new AdminPolicyStore();
	setContext(POLICY_CONTEXT_KEY, store);
	return store;
}

export function getAdminPolicyContext(): AdminPolicyStore {
	return getContext<AdminPolicyStore>(POLICY_CONTEXT_KEY);
}
