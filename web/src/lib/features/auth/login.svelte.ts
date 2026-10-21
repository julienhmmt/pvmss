import { post, ApiRequestError } from '#lib/shared/api/client.js';
import { fetchClusterOptions, type ClusterOption } from '#lib/shared/clusters.js';
import { m } from '#lib/paraglide/messages.js';

/** Server error codes (server/internal/httpapi/auth.go) mapped to translated
 * copy. Falls back to a generic translated message for unknown codes so the
 * raw English server message (e.g. "invalid credentials") is never shown. */
const KNOWN_ERROR_CODES: Partial<Record<string, () => string>> = {
	invalid_credentials: m['login.error.invalidCredentials'],
	invalid_request: m['login.error.invalidRequest'],
	cluster_required: m['login.error.clusterRequired'],
	invalid_cluster: m['login.error.invalidCluster'],
	cluster_unavailable: m['login.error.clusterUnavailable'],
	realm_unsupported: m['login.error.realmUnsupported']
};

function translateError(error: unknown, fallback: () => string): string {
	if (error instanceof ApiRequestError) {
		const translated = KNOWN_ERROR_CODES[error.code];
		if (translated) return translated();
	}
	return fallback();
}

export type LoginProvider = 'pve' | 'local';
export type LoginRealm = 'pve' | 'pam';

export interface Principal {
	username: string;
	displayName: string;
	pool: string;
	isAdmin: boolean;
	cluster: string;
	clusterDisplayName: string;
	provider: 'pve' | 'local' | 'bootstrap';
	isOwner: boolean;
}

export class LoginForm {
	username = $state.raw('');
	password = $state.raw('');
	// Local mode: an empty username means the bootstrap administrator ("admin"),
	// which keeps the previous single-password form working.
	localUsername = $state.raw('');
	provider = $state.raw<LoginProvider>('pve');
	realm = $state.raw<LoginRealm>('pve');
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
				const request: { username?: string; password: string } = { password: this.password };
				const username = this.localUsername.trim();
				if (username !== '') request.username = username;
				return await post<Principal>('/api/v1/auth/admin-login', request);
			}
			if (this.pveDisabled) {
				this.error = m['login.error.clusterUnavailable']();
				return null;
			}
			const username = this.username;
			const request: { username: string; password: string; cluster?: string; realm?: LoginRealm } = {
				username,
				password: this.password
			};
			if (this.cluster !== '') request.cluster = this.cluster;
			// Only send a realm for a bare username: an explicit @realm in the
			// field wins and the server rejects a contradiction.
			if (!username.includes('@')) request.realm = this.realm;
			return await post<Principal>('/api/v1/auth/login', request);
		} catch (error: unknown) {
			this.error = translateError(error, m['login.error.generic']);
			return null;
		} finally {
			this.loading = false;
		}
	}
}
