import { get, put, ApiRequestError } from '$lib/shared/api/client';
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

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.policy = await get<AdminPolicy>('/api/v1/admin/policy?cluster=default');
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
			this.policy = await put<AdminPolicy>('/api/v1/admin/policy', { cluster: 'default', ...patch });
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
