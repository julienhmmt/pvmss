import { m } from '$lib/paraglide/messages.js';

/** One thing an administrator should act on, computed by the server. */
export interface DashboardAlert {
	kind: 'cluster_unreachable' | 'node_offline' | 'node_cpu' | 'node_memory' | 'storage_full' | 'pool_at_quota';
	severity: 'critical' | 'warning';
	cluster: string;
	clusterKey?: string;
	subject?: string;
	percent?: number;
}

type AdminPath = '/admin/clusters' | '/admin/storages' | '/admin/policy';

/** A parameterized route plus its params, for alerts that deep-link. */
interface NodeDetailRoute {
	route: '/admin/nodes/[cluster]/[node]';
	params: { cluster: string; node: string };
}

/** Where an alert links: a plain path, or a route + params to resolve. */
export type AlertTarget = AdminPath | NodeDetailRoute;

/** The page where the alert's cause is fixed. Node alerts deep-link to the
 * node detail page; the rest land on the matching admin list. */
export function alertHref(alert: DashboardAlert): AlertTarget {
	switch (alert.kind) {
		case 'node_offline':
		case 'node_cpu':
		case 'node_memory':
			return {
				route: '/admin/nodes/[cluster]/[node]',
				params: { cluster: alert.clusterKey ?? alert.cluster, node: alert.subject ?? '' }
			};
		case 'cluster_unreachable':
			return '/admin/clusters';
		case 'storage_full':
			return '/admin/storages';
		case 'pool_at_quota':
			return '/admin/policy';
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
