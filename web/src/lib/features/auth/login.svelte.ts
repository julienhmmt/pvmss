import { post } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';

export type LoginProvider = 'pve' | 'local';

export interface Principal {
	username: string;
	pool: string;
	isAdmin: boolean;
	cluster: string;
	clusterDisplayName: string;
}

export class LoginForm {
	username = $state.raw('');
	password = $state.raw('');
	provider = $state.raw<LoginProvider>('pve');
	clusters = $state.raw<ClusterOption[]>([]);
	cluster = $state.raw('');
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async loadClusters(): Promise<void> {
		try {
			this.clusters = await fetchClusterOptions();
			const first = this.clusters[0];
			if (first && this.cluster === '') this.cluster = first.name;
		} catch {
			this.clusters = [];
		}
	}

	get selectedCluster(): ClusterOption | undefined {
		return this.clusters.find((option) => option.name === this.cluster);
	}

	async submit(): Promise<Principal | null> {
		this.loading = true;
		this.error = null;
		try {
			if (this.provider === 'local') {
				return await post<Principal>('/api/v1/auth/admin-login', { password: this.password });
			}
			const request: { username: string; password: string; cluster?: string } = {
				username: this.username,
				password: this.password
			};
			if (this.cluster !== '') request.cluster = this.cluster;
			return await post<Principal>('/api/v1/auth/login', request);
		} catch (error: unknown) {
			this.error = error instanceof Error ? error.message : 'unable to sign in';
			return null;
		} finally {
			this.loading = false;
		}
	}

	async signInOIDC(): Promise<boolean> {
		if (this.cluster === '') {
			this.error = 'choose a cluster before using OIDC';
			return false;
		}
		try {
			await post<void>('/api/v1/auth/oidc', { cluster: this.cluster });
			return true;
		} catch (error: unknown) {
			this.error = error instanceof Error ? error.message : 'OIDC sign-in is unavailable';
			return false;
		}
	}
}
