import { getContext, setContext } from 'svelte';
import type { NotificationsState } from '#lib/features/notifications/notifications.svelte.js';

/**
 * ChromeState - owns the app-shell layout state: the sidebar drawer (open on
 * viewports < 900px, forced closed on desktop) and the command palette.
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

	/** Command palette (ticket 06), opened by Cmd/Ctrl+K or a header button. */
	paletteOpen = $state(false);

	openPalette(): void {
		this.paletteOpen = true;
	}

	closePalette(): void {
		this.paletteOpen = false;
	}

	/** "Stop motion": freezes every animation app-wide (`html[data-motion-off]`), remembered per browser. */
	motionOff = $state(readMotionOff());

	toggleMotion(): void {
		this.motionOff = !this.motionOff;
		document.documentElement.toggleAttribute('data-motion-off', this.motionOff);
		try {
			if (this.motionOff) localStorage.setItem(MOTION_STORAGE_KEY, 'off');
			else localStorage.removeItem(MOTION_STORAGE_KEY);
		} catch {
			// Storage blocked: the choice still holds for this page view.
		}
	}

	/** Pool users' notifications, owned by AppHeader (mounted on every viewport), shown by the sidebar too. */
	notifications = $state.raw<NotificationsState | null>(null);
}

/** Also read by the first-paint script in app.html. */
export const MOTION_STORAGE_KEY = 'pvmss-motion-v1';

function readMotionOff(): boolean {
	try {
		return localStorage.getItem(MOTION_STORAGE_KEY) === 'off';
	} catch {
		return false;
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
