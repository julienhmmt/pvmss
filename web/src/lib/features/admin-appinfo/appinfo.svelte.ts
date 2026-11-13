import { get, ApiRequestError } from '#lib/shared/api/client.js';
import { setContext, getContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '#lib/paraglide/messages.js';

export interface ConfigField {
	name: string;
	value: string | null;
	redacted: boolean;
}

export interface ClusterHealth {
	name: string;
	refreshedAt: string;
	lastRefreshSucceeded: boolean;
}

export interface AppInfo {
	version: string;
	config: ConfigField[];
	clusters: ClusterHealth[];
}

/**
 * AppInfoStore manages the admin app info view. API responses are $state.raw
 * - they are API data, not form edits (constitution VII). One store instance
 * per admin appinfo page, via context.
 */
export class AppInfoStore {
	info = $state.raw<AppInfo | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	/** Names of secret fields whose value is currently shown. */
	revealedSecrets = new SvelteSet<string>();
	/** Whether the last load included real secret values (?reveal=1). */
	private secretsLoaded = $state.raw(false);

	async load(reveal = this.secretsLoaded): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.info = await get<AppInfo>(`/api/v1/admin/appinfo${reveal ? '?reveal=1' : ''}`);
			this.secretsLoaded = reveal;
			if (!reveal) this.revealedSecrets.clear();
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.appinfo.loadError']();
		} finally {
			this.loading = false;
		}
	}

	/** Shows a secret field's real value. Call only after the admin
	 * confirmed the sensitive-value warning. The first reveal refetches
	 * appinfo with ?reveal=1. */
	async reveal(name: string): Promise<void> {
		if (!this.secretsLoaded) await this.load(true);
		this.revealedSecrets.add(name);
	}

	/** Hides a previously revealed secret field. */
	hide(name: string): void {
		this.revealedSecrets.delete(name);
	}
}

const APPINFO_CONTEXT_KEY = Symbol('admin-appinfo');

export function setAppInfoContext(): AppInfoStore {
	const store = new AppInfoStore();
	setContext(APPINFO_CONTEXT_KEY, store);
	return store;
}

export function getAppInfoContext(): AppInfoStore {
	return getContext<AppInfoStore>(APPINFO_CONTEXT_KEY);
}
