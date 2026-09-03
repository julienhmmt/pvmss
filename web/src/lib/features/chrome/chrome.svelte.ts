import { getContext, setContext } from 'svelte';

/**
 * ChromeState — owns the app-shell layout state: the sidebar drawer (open on
 * viewports < 900px, forced closed on desktop).
 *
 * Constitution VII: instantiated once in +layout.svelte and provided via
 * context, not a module singleton. The server is untouched by this feature.
 */
export class ChromeState {
	sidebarOpen = $state(false);

	openSidebar(): void {
		this.sidebarOpen = true;
	}

	closeSidebar(): void {
		this.sidebarOpen = false;
	}
}

const CHROME_CONTEXT_KEY = Symbol('chrome-layout');

export function setChromeContext(): ChromeState {
	const state = new ChromeState();
	setContext(CHROME_CONTEXT_KEY, state);
	return state;
}

export function getChromeContext(): ChromeState {
	return getContext<ChromeState>(CHROME_CONTEXT_KEY);
}
