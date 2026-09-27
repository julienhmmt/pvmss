import { m } from '$lib/paraglide/messages.js';

/** One thing an administrator should act on, computed by the server. */
export interface DashboardAlert {
	kind: 'cluster_unreachable' | 'node_offline' | 'node_cpu' | 'node_memory' | 'storage_full' | 'pool_at_quota';
	severity: 'critical' | 'warning';
	cluster: string;
	subject?: string;
	percent?: number;
}

type AdminPath = '/admin/clusters' | '/nodes' | '/admin/storages' | '/admin/policy';

const HREF: Record<DashboardAlert['kind'], AdminPath> = {
	cluster_unreachable: '/admin/clusters',
	node_offline: '/nodes',
	node_cpu: '/nodes',
	node_memory: '/nodes',
	storage_full: '/admin/storages',
	pool_at_quota: '/admin/policy'
};

/** The page where the alert's cause is fixed. */
export function alertHref(alert: DashboardAlert): AdminPath {
	return HREF[alert.kind];
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
