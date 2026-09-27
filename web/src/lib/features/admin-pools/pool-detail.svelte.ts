import { del, get, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions } from '$lib/shared/clusters';
import { getContext, setContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

/** PoolMemberVM is one member VM row on the pool detail page. */
export interface PoolMemberVM {
	vmid: number;
	name: string;
	node: string;
	status: string;
	uptimeSeconds: number;
	cpuCores: number;
	memoryBytes: number;
	diskBytes: number;
	ipAddresses: string[];
}

/** PoolActivityEntry is one merged audit row for the pool: the pool user's
 *  own actions plus admin actions targeting the pool. */
export interface PoolActivityEntry {
	id: number;
	actor: string;
	cluster: string;
	vmid: number | null;
	action: string;
	timestamp: string;
	targetType: string;
	targetId: string;
	detail: string;
	severity: string;
}

/** PoolDetail is the aggregate returned by GET /api/v1/admin/pools/{name}. */
export interface PoolDetail {
	name: string;
	username: string;
	comment: string;
	cluster: string;
	managed: boolean;
	createdAt?: string;
	quota: { used: number; allowed: number };
	vms: PoolMemberVM[];
	activity: PoolActivityEntry[];
}

interface DeletePoolResponse {
	status: string;
	userDeleted: boolean;
}

const ADMIN_POOL_DETAIL_CONTEXT_KEY = Symbol('admin-pool-detail');

/** Manages the admin pool detail view: one pool's identity, managed marker,
 *  member VMs, quota usage, recent activity, and deletion. */
export class PoolDetailStore {
	detail = $state.raw<PoolDetail | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	notFound = $state.raw(false);
	deleting = $state.raw(false);
	deleteError = $state.raw<string | null>(null);
	cluster = $state('');

	/** Loads the named pool. The cluster comes from the list page's
	 *  ?cluster= hint; without one the first available cluster is used. */
	async load(name: string, clusterHint: string = ''): Promise<void> {
		const cluster = clusterHint || (await this.firstCluster());
		this.cluster = cluster;
		this.loading = true;
		this.error = null;
		this.notFound = false;
		try {
			this.detail = await get<PoolDetail>(
				`/api/v1/admin/pools/${encodeURIComponent(name)}?cluster=${encodeURIComponent(cluster)}`
			);
		} catch (err) {
			this.notFound = err instanceof ApiRequestError && err.status === 404;
			this.error = err instanceof ApiRequestError ? err.message : m['admin.poolDetail.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async refresh(): Promise<void> {
		if (this.detail) {
			await this.load(this.detail.name, this.cluster);
		}
	}

	async remove(): Promise<void> {
		if (!this.detail) return;
		this.deleting = true;
		this.deleteError = null;
		try {
			await del<DeletePoolResponse>(
				`/api/v1/admin/pools/${encodeURIComponent(this.detail.name)}?cluster=${encodeURIComponent(this.cluster)}`
			);
		} catch (err) {
			this.deleteError = err instanceof ApiRequestError ? err.message : m['admin.pools.deleteError']();
			throw err;
		} finally {
			this.deleting = false;
		}
	}

	private async firstCluster(): Promise<string> {
		try {
			const options = await fetchClusterOptions();
			return options[0]?.name ?? '';
		} catch {
			return '';
		}
	}
}

export function setAdminPoolDetailContext(): PoolDetailStore {
	const store = new PoolDetailStore();
	setContext(ADMIN_POOL_DETAIL_CONTEXT_KEY, store);
	return store;
}

export function getAdminPoolDetailContext(): PoolDetailStore {
	return getContext<PoolDetailStore>(ADMIN_POOL_DETAIL_CONTEXT_KEY);
}
