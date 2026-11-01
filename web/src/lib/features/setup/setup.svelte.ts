import { getContext, setContext } from 'svelte';
import { fetchSetupStatus, type SetupStatus } from './setup.js';

/** SetupStore exposes the first-run status fetched once by the shell. The
 * layout guards on it: `required` sends every page to /setup, and /setup
 * hands visitors back once nothing is needed. */
export class SetupStore {
	status = $state.raw<SetupStatus | null>(null);
	loading = $state.raw(false);

	async load(): Promise<void> {
		this.loading = true;
		try {
			this.status = await fetchSetupStatus();
		} catch {
			// A failed status read must not strand the app behind a false
			// "setup required" guard - stay undecided (null) and retry never.
			this.status = null;
		} finally {
			this.loading = false;
		}
	}
}

const SETUP_CONTEXT_KEY = Symbol('setup');

export function setSetupContext(): SetupStore {
	const store = new SetupStore();
	setContext(SETUP_CONTEXT_KEY, store);
	return store;
}

export function getSetupContext(): SetupStore {
	return getContext<SetupStore>(SETUP_CONTEXT_KEY);
}
