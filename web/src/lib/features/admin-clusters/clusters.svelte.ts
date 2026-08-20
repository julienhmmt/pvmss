import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';

export interface AdminCluster {
	name: string;
	displayName: string;
	url: string;
	tlsInsecureSkipVerify: boolean;
	tokenId: string;
	tokenSet: boolean;
	oidcEnabled: boolean;
	removedAt: string | null;
	lastTestStatus: 'ok' | 'unreachable' | 'error' | null;
	lastTestAt: string | null;
	lastTestMessage: string | null;
	proxmoxVersion: string | null;
	nodeCount: number;
	vmCount: number;
}

export interface ClusterInput {
	name: string;
	url: string;
	tlsInsecureSkipVerify: boolean;
	tokenId: string;
	tokenSecret: string;
}

export interface ClusterTestResult {
	status: 'ok' | 'unreachable' | 'error';
	message?: string;
	proxmoxVersion?: string;
	nodeCount?: number;
	vmCount?: number;
	testedAt: string;
}

export class AdminClustersStore {
	clusters = $state.raw<AdminCluster[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	announce = $state.raw<string | null>(null);
	busy = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.clusters = await get<AdminCluster[]>('/api/v1/admin/clusters');
		} catch (error: unknown) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.clusters.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async create(input: ClusterInput): Promise<boolean> {
		return this.run('create', async () => {
			const created = await post<AdminCluster>('/api/v1/admin/clusters', input);
			this.clusters = [...this.clusters.filter((cluster) => cluster.name !== created.name), created];
			this.announce = m['admin.clusters.added']({ name: created.name });
		});
	}

	async update(name: string, input: Omit<ClusterInput, 'name'>): Promise<boolean> {
		return this.run(`update:${name}`, async () => {
			const updated = await put<AdminCluster>(`/api/v1/admin/clusters/${encodeURIComponent(name)}`, input);
			this.clusters = this.clusters.map((cluster) => (cluster.name === name ? updated : cluster));
			this.announce = m['admin.clusters.updated']({ name });
		});
	}

	async test(name: string): Promise<ClusterTestResult | null> {
		let result: ClusterTestResult | null = null;
		await this.run(`test:${name}`, async () => {
			result = await post<ClusterTestResult>(`/api/v1/admin/clusters/${encodeURIComponent(name)}/test`);
			await this.load();
			this.announce = m['admin.clusters.tested']({ name, status: result?.status ?? 'tested' });
		});
		return result;
	}

	async toggleOIDC(name: string, enabled: boolean): Promise<boolean> {
		return this.run(`oidc:${name}`, async () => {
			await post<{ name: string; oidcEnabled: boolean }>(`/api/v1/admin/clusters/${encodeURIComponent(name)}/oidc`, { enabled });
			this.clusters = this.clusters.map((cluster) => (cluster.name === name ? { ...cluster, oidcEnabled: enabled } : cluster));
			this.announce = enabled ? m['admin.clusters.oidcEnabled']({ name }) : m['admin.clusters.oidcDisabled']({ name });
		});
	}

	async remove(name: string): Promise<boolean> {
		return this.run(`remove:${name}`, async () => {
			await del<{ status: string }>(`/api/v1/admin/clusters/${encodeURIComponent(name)}`);
			this.clusters = this.clusters.filter((cluster) => cluster.name !== name);
			this.announce = m['admin.clusters.removed']({ name });
		});
	}

	async run(key: string, operation: () => Promise<void>): Promise<boolean> {
		this.busy = key;
		this.error = null;
		try {
			await operation();
			return true;
		} catch (error: unknown) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.clusters.operationFailed']();
			return false;
		} finally {
			this.busy = null;
		}
	}
}
