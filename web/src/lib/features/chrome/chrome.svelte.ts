import { getContext, setContext } from 'svelte';

/**
 * ChromeState — owns the app-shell layout state: the sidebar drawer (open on
 * viewports < 900px, forced closed on desktop) and the activity drawer.
 *
 * Constitution VII: instantiated once in +layout.svelte and provided via
 * context, not a module singleton. The server is untouched by this feature.
 */
export class ChromeState {
	sidebarOpen = $state(false);
	activityOpen = $state(false);

	openSidebar(): void {
		this.sidebarOpen = true;
	}

	closeSidebar(): void {
		this.sidebarOpen = false;
	}

	/** Forces the drawer closed — called when the viewport crosses 900px upward. */
	closeSidebarOnDesktop(): void {
		this.sidebarOpen = false;
	}

	openActivity(): void {
		this.activityOpen = true;
	}

	closeActivity(): void {
		this.activityOpen = false;
	}

	toggleActivity(): void {
		this.activityOpen = !this.activityOpen;
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
