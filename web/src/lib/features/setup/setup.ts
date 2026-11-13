import { get, post, ApiRequestError } from '#lib/shared/api/client.js';
import type { AdminCluster, ClusterInput, ClusterTlsInput } from '#lib/features/admin-clusters/clusters.svelte.js';
import type { Principal } from '#lib/features/auth/login.svelte.js';
import { m } from '#lib/paraglide/messages.js';

/** Server error codes (server/internal/httpapi/setup.go,
 * admin_clusters.go, auth.go) mapped to translated copy so the raw English
 * server message is never shown. */
const KNOWN_ERROR_CODES: Partial<Record<string, () => string>> = {
	invalid_request: m['setup.error.invalidRequest'],
	invalid_username: m['setup.error.invalidUsername'],
	weak_password: m['setup.error.weakPassword'],
	username_taken: m['setup.error.usernameTaken'],
	setup_complete: m['setup.error.setupComplete'],
	invalid_credentials: m['setup.error.invalidCredentials'],
	invalid_cluster_name: m['setup.error.invalidClusterName'],
	duplicate_cluster: m['setup.error.duplicateCluster'],
	invalid_tls: m['setup.error.invalidTls']
};

/** Maps a failed setup call to translated copy; falls back to generic. */
export function translateSetupError(error: unknown): string {
	if (error instanceof ApiRequestError) {
		const translated = KNOWN_ERROR_CODES[error.code];
		if (translated) return translated();
	}
	return m['setup.error.generic']();
}

/** GET /api/v1/setup response: which first-run steps remain. */
export interface SetupStatus {
	required: boolean;
	needsAdmin: boolean;
	needsCluster: boolean;
}

/** POST /api/v1/admin/clusters/probe response: connection outcome plus the
 * name the target cluster reports, used as the wizard's name suggestion. */
export interface ClusterProbe {
	status: 'ok' | 'unreachable' | 'error';
	clusterName?: string;
	message?: string;
	tlsStatus?: 'verified' | 'skipped' | 'failed' | 'unknown';
	proxmoxVersion?: string;
	nodeCount?: number;
	vmCount?: number;
	testedAt: string;
}

export interface SetupAdminInput {
	username: string;
	password: string;
}

export interface SetupClusterInput {
	url: string;
	tokenId: string;
	tokenSecret: string;
	tlsInsecureSkipVerify: boolean;
}

/** Immutable logical cluster name, mirrors the server's ^[a-z0-9-]+$ rule. */
const CLUSTER_NAME_PATTERN = /^[a-z0-9-]+$/;

/** Returns whether name is a valid immutable cluster identifier. */
export function isValidClusterName(name: string): boolean {
	return CLUSTER_NAME_PATTERN.test(name);
}

/** Turns the cluster's reported display name into a suggested identifier:
 * lowercase, non [a-z0-9] runs collapse into a single dash, edges trimmed.
 * The split/filter/join trims edge dashes in one linear pass - no edge-trim
 * regex (S5852). */
export function suggestClusterName(displayName: string): string {
	return displayName
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.split('-')
		.filter(Boolean)
		.join('-');
}

/** The wizard's current step, derived from the server-reported status and the
 * local session. 'admin' creates the first account, 'signin' authenticates an
 * existing one, 'cluster' registers the first cluster. */
export function setupStep(status: SetupStatus, signedIn: boolean): 'admin' | 'signin' | 'cluster' | 'done' {
	if (!status.needsAdmin && !status.needsCluster) return 'done';
	if (!signedIn) return status.needsAdmin ? 'admin' : 'signin';
	if (status.needsCluster) return 'cluster';
	return 'admin';
}

export function fetchSetupStatus(): Promise<SetupStatus> {
	return get<SetupStatus>('/api/v1/setup');
}

/** Creates the first local administrator. Refused (403 setup_complete) once
 * any administrator exists. */
export function createSetupAdmin(input: SetupAdminInput): Promise<{ username: string }> {
	return post<{ username: string }>('/api/v1/setup/admin', input);
}

/** Signs in through the admin endpoint; an empty username selects the
 * bootstrap administrator. */
export function adminLogin(username: string, password: string): Promise<Principal> {
	const request: { username?: string; password: string } = { password };
	const trimmed = username.trim();
	if (trimmed !== '') request.username = trimmed;
	return post<Principal>('/api/v1/auth/admin-login', request);
}

/** Tests unsaved connection settings; the result carries the suggested name. */
export function probeCluster(input: SetupClusterInput): Promise<ClusterProbe> {
	return post<ClusterProbe>('/api/v1/admin/clusters/probe', input);
}

export function createSetupCluster(input: ClusterInput): Promise<AdminCluster> {
	return post<AdminCluster>('/api/v1/admin/clusters', input);
}

/** Probe and create take the same connection fields; the wizard fills the
 * immutable name and empty TLS material around them. */
export function clusterInputFromSetup(name: string, displayName: string, input: SetupClusterInput): ClusterInput {
	const tls: ClusterTlsInput = {};
	return {
		name: name.trim(),
		displayName: displayName.trim(),
		url: input.url.trim(),
		tokenId: input.tokenId.trim(),
		tokenSecret: input.tokenSecret,
		tlsInsecureSkipVerify: input.tlsInsecureSkipVerify,
		tls
	};
}
