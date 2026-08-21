import { get, put, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { getContext, setContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface Gabarit {
	maxSockets: number;
	maxCores: number;
	maxMemoryMB: number;
	maxDiskPerVmGb: number;
	maxNetworkCards: number;
	maxSnapshots: number;
	allowCustomYaml: boolean;
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
	allowCustomYaml?: boolean;
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
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	saved = $state.raw(false);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			const first = this.clusterOptions[0];
			if (first && (this.clusterOptions.length === 1 || !this.clusterOptions.some((option) => option.name === this.cluster))) {
				this.cluster = first.name;
			}
		} catch (error: unknown) {
			this.error = error instanceof ApiRequestError ? error.message : m['policy.loadError']();
		}
	}

	setCluster(value: string): void {
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		try {
			this.policy = await get<AdminPolicy>(`/api/v1/admin/policy?cluster=${encodeURIComponent(this.cluster)}`);
		} catch (error: unknown) {
			this.error = error instanceof ApiRequestError ? error.message : m['policy.loadError']();
		} finally {
			this.loading = false;
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
