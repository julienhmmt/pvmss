import { del, get, patch, put, ApiRequestError } from '#lib/shared/api/client.js';
import { fetchClusterOptions } from '#lib/shared/clusters.js';
import { getContext, setContext } from 'svelte';
import { STALE_REFRESH_MS } from '#lib/shared/visibility-refresh.js';
import { m } from '#lib/paraglide/messages.js';
import {
	fetchQuotaOverrides,
	removeQuotaOverride,
	setQuotaOverride,
	type QuotaOverride
} from '#lib/features/quota-overrides/quota-overrides.js';

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
	/** Created at a first SSO login: no password to reset. */
	sso?: boolean;
	createdAt?: string;
	quota: { used: number; allowed: number };
	vms: PoolMemberVM[];
	activity: PoolActivityEntry[];
}

interface DeletePoolResponse {
	status: string;
	userDeleted: boolean;
}

/** ResetPoolCredentials is the one-time payload returned by the password
 *  reset: the new secret is never stored server-side and only lives in the
 *  response body. */
export interface ResetPoolCredentials {
	username: string;
	password: string;
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
	savingComment = $state.raw(false);
	commentError = $state.raw<string | null>(null);
	/** The pool's per-pool cap, or null when it uses the global quota. */
	override = $state.raw<QuotaOverride | null>(null);
	overrideSaving = $state.raw(false);
	overrideError = $state.raw<string | null>(null);
	resetting = $state.raw(false);
	resetError = $state.raw<string | null>(null);
	cluster = $state('');
	/** `Date.now()` of the last completed load - `refreshIfStale()` uses it to
	 * skip reloads fresher than the interval. */
	lastLoadedAt = 0;

	/** Reloads when the last completed load is older than `minIntervalMs` -
	 * the visibility/poll hook calls this instead of loading on every tick. */
	async refreshIfStale(minIntervalMs: number = STALE_REFRESH_MS): Promise<void> {
		if (Date.now() - this.lastLoadedAt <= minIntervalMs) return;
		await this.refresh();
	}

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
		if (this.detail) {
			this.lastLoadedAt = Date.now();
			await this.loadOverride(this.detail.name, cluster);
		}
	}

	/** Reads the pool's override; a failure is shown on its own so the page still renders. */
	private async loadOverride(name: string, cluster: string): Promise<void> {
		try {
			const overrides = await fetchQuotaOverrides(cluster);
			this.override = overrides.find((item) => item.pool === name) ?? null;
			this.overrideError = null;
		} catch (err) {
			this.override = null;
			this.overrideError = err instanceof ApiRequestError ? err.message : m['admin.poolQuota.loadError']();
		}
	}

	/** Sets the pool's cap; throws so the caller can keep the input open. */
	async saveOverride(maxVms: number): Promise<void> {
		if (!this.detail) return;
		this.overrideSaving = true;
		this.overrideError = null;
		try {
			await setQuotaOverride(this.cluster, this.detail.name, maxVms);
			await this.refresh();
		} catch (err) {
			this.overrideError = err instanceof ApiRequestError ? err.message : m['admin.poolQuota.saveError']();
			throw err;
		} finally {
			this.overrideSaving = false;
		}
	}

	/** Removes the pool's cap so it falls back to the global quota. */
	async removeOverride(): Promise<void> {
		if (!this.detail) return;
		this.overrideSaving = true;
		this.overrideError = null;
		try {
			await removeQuotaOverride(this.cluster, this.detail.name);
			await this.refresh();
		} catch (err) {
			this.overrideError = err instanceof ApiRequestError ? err.message : m['admin.poolQuota.removeError']();
			throw err;
		} finally {
			this.overrideSaving = false;
		}
	}

	async refresh(): Promise<void> {
		if (this.detail) {
			await this.load(this.detail.name, this.cluster);
		}
	}

	/** Replaces the pool comment; throws so the caller can keep the editor open. */
	async updateComment(comment: string): Promise<void> {
		if (!this.detail) return;
		this.savingComment = true;
		this.commentError = null;
		try {
			const saved = await patch<{ comment: string }>(
				`/api/v1/admin/pools/${encodeURIComponent(this.detail.name)}?cluster=${encodeURIComponent(this.cluster)}`,
				{ comment }
			);
			this.detail = { ...this.detail, comment: saved.comment };
		} catch (err) {
			this.commentError = err instanceof ApiRequestError ? err.message : m['admin.poolDetail.commentError']();
			throw err;
		} finally {
			this.savingComment = false;
		}
	}

	/** Rotates the pool user's sign-in password. An empty password asks the
	 *  server to generate one; the resolved credential is shown once. Throws
	 *  so the dialog can keep the form open on failure. */
	async resetPassword(password: string = ''): Promise<ResetPoolCredentials> {
		if (!this.detail) throw new Error('pool detail not loaded');
		this.resetting = true;
		this.resetError = null;
		try {
			return await put<ResetPoolCredentials>(
				`/api/v1/admin/pools/${encodeURIComponent(this.detail.name)}/password?cluster=${encodeURIComponent(this.cluster)}`,
				{ password }
			);
		} catch (err) {
			this.resetError = err instanceof ApiRequestError ? err.message : m['admin.poolDetail.resetError']();
			throw err;
		} finally {
			this.resetting = false;
		}
	}

	/** Resolves to whether the Proxmox account was deleted too. */
	async remove(): Promise<boolean> {
		if (!this.detail) return true;
		this.deleting = true;
		this.deleteError = null;
		try {
			const result = await del<DeletePoolResponse>(
				`/api/v1/admin/pools/${encodeURIComponent(this.detail.name)}?cluster=${encodeURIComponent(this.cluster)}`
			);
			return result.userDeleted;
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
