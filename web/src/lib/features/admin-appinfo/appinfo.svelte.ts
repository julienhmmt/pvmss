import { get, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

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
 * — they are API data, not form edits (constitution VII). One store instance
 * per admin appinfo page, via context.
 */
export class AppInfoStore {
	info = $state.raw<AppInfo | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.info = await get<AppInfo>('/api/v1/admin/appinfo');
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.appinfo.loadError']();
		} finally {
			this.loading = false;
		}
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
