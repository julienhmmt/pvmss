import { post, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { m } from '$lib/paraglide/messages.js';

/** Server error codes (server/internal/httpapi/auth.go) mapped to translated
 * copy. Falls back to a generic translated message for unknown codes so the
 * raw English server message (e.g. "invalid credentials") is never shown. */
const KNOWN_ERROR_CODES: Partial<Record<string, () => string>> = {
	invalid_credentials: m['login.error.invalidCredentials'],
	invalid_request: m['login.error.invalidRequest'],
	cluster_required: m['login.error.clusterRequired'],
	invalid_cluster: m['login.error.invalidCluster'],
	cluster_unavailable: m['login.error.clusterUnavailable'],
	not_found: m['login.error.oidcNotEnabled']
};

function translateError(error: unknown, fallback: () => string): string {
	if (error instanceof ApiRequestError) {
		const translated = KNOWN_ERROR_CODES[error.code];
		if (translated) return translated();
	}
	return fallback();
}

export type LoginProvider = 'pve' | 'local';

export interface Principal {
	username: string;
	displayName: string;
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
	pveDisabled = $state.raw(false);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	setPveDisabled(value: boolean): void {
		this.pveDisabled = value;
	}

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
			if (this.pveDisabled) {
				this.error = m['login.error.clusterUnavailable']();
				return null;
			}
			const username = this.username.includes('@') ? this.username : `${this.username}@pve`;
			const request: { username: string; password: string; cluster?: string } = {
				username,
				password: this.password
			};
			if (this.cluster !== '') request.cluster = this.cluster;
			return await post<Principal>('/api/v1/auth/login', request);
		} catch (error: unknown) {
			this.error = translateError(error, m['login.error.generic']);
			return null;
		} finally {
			this.loading = false;
		}
	}

	async signInOIDC(): Promise<boolean> {
		if (this.cluster === '') {
			this.error = m['login.error.clusterRequired']();
			return false;
		}
		try {
			await post<void>('/api/v1/auth/oidc', { cluster: this.cluster });
			return true;
		} catch (error: unknown) {
			this.error = translateError(error, m['login.error.oidcUnavailable']);
			return false;
		}
	}
}
