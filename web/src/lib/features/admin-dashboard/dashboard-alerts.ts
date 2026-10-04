import { resolve } from '$app/paths';
import { m } from '$lib/paraglide/messages.js';

/** One thing an administrator should act on, computed by the server. */
export interface DashboardAlert {
	kind:
		| 'cluster_unreachable'
		| 'node_offline'
		| 'node_offline_disabled'
		| 'node_cpu'
		| 'node_memory'
		| 'storage_full'
		| 'pool_at_quota';
	severity: 'critical' | 'warning' | 'info';
	cluster: string;
	clusterKey?: string;
	subject?: string;
	percent?: number;
}

/** Appends non-empty params as a query string, or returns the path alone. */
function withQuery(path: string, query: Record<string, string>): string {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(query)) {
		if (value !== '') params.set(key, value);
	}
	const qs = params.toString();
	return qs === '' ? path : `${path}?${qs}`;
}

/** The URL where the alert's cause is fixed. Node alerts deep-link to the
 * node detail page; storage and pool alerts land on the matching admin list
 * with a ?cluster (and ?search) hint so it opens already filtered. */
export function alertHref(alert: DashboardAlert): string {
	const clusterKey = alert.clusterKey ?? alert.cluster;
	switch (alert.kind) {
		case 'node_offline':
		case 'node_offline_disabled':
		case 'node_cpu':
		case 'node_memory':
			return resolve('/admin/nodes/[cluster]/[node]', { cluster: clusterKey, node: alert.subject ?? '' });
		case 'cluster_unreachable':
			return resolve('/admin/clusters');
		case 'storage_full':
			return withQuery(resolve('/admin/storages'), { cluster: clusterKey, search: alert.subject ?? '' });
		case 'pool_at_quota':
			return withQuery(resolve('/admin/policy'), { cluster: clusterKey });
	}
}

export function alertMessage(alert: DashboardAlert): string {
	const subject = alert.subject ?? '';
	const percent = alert.percent ?? 0;
	switch (alert.kind) {
		case 'cluster_unreachable':
			return m['admin.dashboard.alert.clusterUnreachable']({ cluster: alert.cluster });
		case 'node_offline':
			return m['admin.dashboard.alert.nodeOffline']({ subject });
		case 'node_offline_disabled':
			return m['admin.dashboard.alert.nodeOfflineDisabled']({ subject });
		case 'node_cpu':
			return m['admin.dashboard.alert.nodeCpu']({ subject, percent });
		case 'node_memory':
			return m['admin.dashboard.alert.nodeMemory']({ subject, percent });
		case 'storage_full':
			return m['admin.dashboard.alert.storageFull']({ subject, percent });
		case 'pool_at_quota':
			return m['admin.dashboard.alert.poolAtQuota']({ subject });
	}
}

/** Bar tone for a usage percentage, on the storage alert thresholds. */
export function usageTone(percent: number): 'success' | 'warning' | 'destructive' {
	if (percent >= 95) return 'destructive';
	if (percent >= 85) return 'warning';
	return 'success';
}
