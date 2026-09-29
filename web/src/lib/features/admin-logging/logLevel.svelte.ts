import { get, put, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export const LOG_LEVELS = ['debug', 'info', 'warn', 'error'] as const;
export type LogLevel = (typeof LOG_LEVELS)[number];

interface LogLevelState {
	level: LogLevel;
	default: LogLevel;
}

/**
 * LogLevelStore reads and changes the server's live log level. The change is
 * not persisted server-side: a restart returns to the startup default.
 */
export class LogLevelStore {
	level = $state.raw<LogLevel | null>(null);
	defaultLevel = $state.raw<LogLevel | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	saved = $state.raw(false);

	get isDefault(): boolean {
		return this.level !== null && this.level === this.defaultLevel;
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.apply(await get<LogLevelState>('/api/v1/admin/ops/log-level'));
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.logLevel.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async set(level: LogLevel): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.saved = false;
		try {
			this.apply(await put<LogLevelState>('/api/v1/admin/ops/log-level', { level }));
			this.saved = true;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.logLevel.saveError']();
		} finally {
			this.saving = false;
		}
	}

	async reset(): Promise<void> {
		if (this.defaultLevel !== null) {
			await this.set(this.defaultLevel);
		}
	}

	private apply(state: LogLevelState): void {
		this.level = state.level;
		this.defaultLevel = state.default;
	}
}

const LOG_LEVEL_CONTEXT_KEY = Symbol('admin-log-level');

export function setLogLevelContext(): LogLevelStore {
	const store = new LogLevelStore();
	setContext(LOG_LEVEL_CONTEXT_KEY, store);
	return store;
}

export function getLogLevelContext(): LogLevelStore {
	return getContext<LogLevelStore>(LOG_LEVEL_CONTEXT_KEY);
}
