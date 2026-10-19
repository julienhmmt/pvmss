import { get } from '#lib/shared/api/client.js';

export interface ClusterOption {
	name: string;
	displayName: string;
}

const ADMIN_CLUSTER_STORAGE_KEY = 'pvmss.adminCluster';

function adminStorage(): Storage | null {
	try {
		return typeof sessionStorage === 'undefined' ? null : sessionStorage;
	} catch {
		return null;
	}
}

/** Reads the last-selected admin cluster from sessionStorage, or null. */
export function getSavedAdminCluster(): string | null {
	const value = adminStorage()?.getItem(ADMIN_CLUSTER_STORAGE_KEY) ?? null;
	return value && value.trim() !== '' ? value : null;
}

/** Persists the active admin cluster selection in sessionStorage. */
export function saveAdminCluster(cluster: string): void {
	const trimmed = cluster.trim();
	if (trimmed === '') return;
	adminStorage()?.setItem(ADMIN_CLUSTER_STORAGE_KEY, trimmed);
}

/**
 * Resolves the initial active cluster for an Admin page:
 * 1. Explicit URL/caller override (`explicitCluster`) wins if present in `options` and updates sessionStorage.
 * 2. Previously saved cluster in sessionStorage wins if present in `options`.
 * 3. Current store cluster wins if `options.length > 1` and present in `options`.
 * 4. Otherwise falls back to `options[0]?.name ?? 'default'`.
 */
export function resolveInitialAdminCluster(
	options: readonly ClusterOption[],
	currentCluster: string,
	explicitCluster?: string | null
): string {
	if (explicitCluster && options.some((option) => option.name === explicitCluster)) {
		saveAdminCluster(explicitCluster);
		return explicitCluster;
	}
	const saved = getSavedAdminCluster();
	if (saved && options.some((option) => option.name === saved)) {
		return saved;
	}
	if (options.length > 1 && options.some((option) => option.name === currentCluster)) {
		return currentCluster;
	}
	return options[0]?.name ?? (currentCluster || 'default');
}

export async function fetchClusterOptions(): Promise<ClusterOption[]> {
	return get<ClusterOption[]>('/api/v1/auth/clusters');
}
