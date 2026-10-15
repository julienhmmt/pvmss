import { get, ApiRequestError } from '$lib/shared/api/client';
import {
	fetchClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from '$lib/shared/clusters';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

import type { CloudInitDocument } from '$lib/features/admin-cloudinit-templates/cloudInitTemplates.svelte';

export interface BaselineState {
	/** The generated baseline: placed on its own for image VMs without a
	 *  template, and merged under every admin template. */
	generated: string;
	/** The standalone baseline file, command and per-node presence (null
	 *  with documentError when it could not be checked). */
	document: CloudInitDocument | null;
	documentError?: string;
}

/**
 * AdminBaselineStore manages the admin baseline view (issue 07). API
 * responses are $state.raw - they are API data, not form edits (constitution
 * VII). One store instance per admin baseline page, via context.
 */
export class AdminBaselineStore {
	state = $state.raw<BaselineState | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			if (this.clusterOptions.length > 0) {
				this.cluster = resolveInitialAdminCluster(this.clusterOptions, this.cluster);
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.baseline.loadError']();
		}
	}

	/** First paint: fetch the cluster list, then the selected cluster's baseline. */
	async init(): Promise<void> {
		await this.loadClusters();
		await this.load(this.cluster);
	}

	setCluster(value: string): void {
		saveAdminCluster(value);
		this.cluster = value;
		void this.load(value);
	}

	async load(cluster: string): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.state = await get<BaselineState>(`/api/v1/admin/baseline?cluster=${encodeURIComponent(cluster)}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.baseline.loadError']();
		} finally {
			this.loading = false;
		}
	}
}

const ADMIN_BASELINE_KEY = Symbol('admin-baseline');

export function setAdminBaselineContext(): AdminBaselineStore {
	const store = new AdminBaselineStore();
	setContext(ADMIN_BASELINE_KEY, store);
	return store;
}

export function getAdminBaselineContext(): AdminBaselineStore {
	return getContext<AdminBaselineStore>(ADMIN_BASELINE_KEY);
}
