import { get, post, put, del, ApiRequestError } from '#lib/shared/api/client.js';
import { m } from '#lib/paraglide/messages.js';

export interface ClusterTlsInfo {
	caSet: boolean;
	caSubject?: string;
	caNotAfter: string | null;
	nodeFingerprints: string[];
	clientCertSet: boolean;
	clientCertSubject?: string;
	clientCertNotAfter: string | null;
	/** The private key itself is never returned. */
	clientKeySet: boolean;
}

/** Omitted field keeps the stored value; an empty string clears it. */
export interface ClusterTlsInput {
	caPem?: string;
	nodeFingerprints?: string;
	clientCertPem?: string;
	clientKeyPem?: string;
}

export interface AdminCluster {
	name: string;
	displayName: string;
	url: string;
	tlsInsecureSkipVerify: boolean;
	tokenId: string;
	tokenSet: boolean;
	removedAt: string | null;
	lastTestStatus: 'ok' | 'unreachable' | 'error' | null;
	lastTestAt: string | null;
	lastTestMessage: string | null;
	proxmoxVersion: string | null;
	nodeCount: number;
	vmCount: number;
	/** Proxmox storage holding the admin cloud-init files ('' = off). */
	snippetStorage: string;
	cloudInitWriteEnabled: boolean;
	tls?: ClusterTlsInfo;
}

export interface ClusterInput {
	name: string;
	displayName?: string;
	url: string;
	tlsInsecureSkipVerify: boolean;
	tokenId: string;
	tokenSecret: string;
	snippetStorage: string;
	tls: ClusterTlsInput;
}

export interface ClusterTestResult {
	status: 'ok' | 'unreachable' | 'error';
	message?: string;
	tlsStatus?: 'verified' | 'skipped' | 'failed' | 'unknown';
	proxmoxVersion?: string;
	nodeCount?: number;
	vmCount?: number;
	testedAt: string;
}

export interface SnippetStorage {
	name: string;
	node: string;
	type: string;
}

export function statusLabel(status: AdminCluster['lastTestStatus']): string {
	switch (status) {
		case 'ok':
			return m['common.statusOk']();
		case 'unreachable':
			return m['common.statusUnreachable']();
		case 'error':
			return m['common.statusError']();
		default:
			return m['common.untested']();
	}
}

export function statusTone(status: AdminCluster['lastTestStatus']): 'ok' | 'error' | 'off' {
	if (status === 'ok') return 'ok';
	if (status === 'unreachable' || status === 'error') return 'error';
	return 'off';
}

export function statusHint(status: AdminCluster['lastTestStatus']): string | null {
	if (status === 'unreachable') return m['admin.clusters.unreachableHint']();
	if (status === 'error') return m['admin.clusters.errorHint']();
	return null;
}

function tlsAnnouncement(name: string, result: ClusterTestResult | null): string | null {
	switch (result?.tlsStatus) {
		case 'verified':
			return m['admin.clusters.tlsTestVerified']({ name });
		case 'skipped':
			return m['admin.clusters.tlsTestSkipped']({ name });
		case 'failed':
			return m['admin.clusters.tlsTestFailed']({ name, message: result.message ?? '' });
		default:
			return null;
	}
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

	/** loadSnippetStorages fetches the snippet-capable storages for one cluster,
	 *  so the cluster form can offer a picker instead of a free-text field. */
	async loadSnippetStorages(cluster: string): Promise<SnippetStorage[]> {
		return get<SnippetStorage[]>(`/api/v1/admin/snippet-storages?cluster=${encodeURIComponent(cluster)}`);
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
			const tested: ClusterTestResult | null = result;
			this.announce = tlsAnnouncement(name, tested) ?? m['admin.clusters.tested']({ name, status: tested?.status ?? 'tested' });
		});
		return result;
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
