import { del, get, put } from '#lib/shared/api/client.js';

/** A per-pool VM cap that replaces the cluster's global per-pool quota for that pool only. */
export interface QuotaOverride {
	cluster: string;
	pool: string;
	maxVms: number;
	updatedBy: string;
	updatedAt: string;
}

/** Lists overrides; an empty cluster lists every cluster's overrides. */
export async function fetchQuotaOverrides(cluster = ''): Promise<QuotaOverride[]> {
	const query = cluster === '' ? '' : `?cluster=${encodeURIComponent(cluster)}`;
	const response = await get<{ overrides: QuotaOverride[] }>(`/api/v1/admin/quota-overrides${query}`);
	return response.overrides ?? [];
}

/** Creates or replaces the override for one pool. */
export function setQuotaOverride(cluster: string, pool: string, maxVms: number): Promise<QuotaOverride> {
	return put<QuotaOverride>(overridePath(cluster, pool), { maxVms });
}

/** Removes the override so the pool falls back to the global quota. */
export function removeQuotaOverride(cluster: string, pool: string): Promise<void> {
	return del<void>(overridePath(cluster, pool));
}

function overridePath(cluster: string, pool: string): string {
	return `/api/v1/admin/quota-overrides/${encodeURIComponent(cluster)}/${encodeURIComponent(pool)}`;
}
