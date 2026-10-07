import { getContext, setContext } from 'svelte';

export type Theme = 'light' | 'dark';

const SUPPORTED_THEMES: readonly Theme[] = ['light', 'dark'];
const DEFAULT_THEME: Theme = 'light';

/**
 * localStorage key for the persisted theme preference. The version lives in
 * the key name (-v1) rather than inside the value: a future breaking change
 * bumps to -v2 and init() never reads the old key, so a stale value is
 * silently orphaned rather than misapplied (data-model.md).
 */
export const THEME_STORAGE_KEY = 'pvmss-theme-v1';

/**
 * Surface tokens (DESIGN.md) mirrored into <meta name="theme-color"> so the
 * browser chrome matches the page background instead of the static #FF8A33
 * baked into app.html.
 */
export const THEME_COLOR_LIGHT = '#f7f6f4';
export const THEME_COLOR_DARK = '#2a2826';

/**
 * ThemeState owns the light/dark preference: a $state-backed current theme,
 * persisted under a versioned localStorage key, applied by toggling the
 * `dark` class on <html> (constitution X: the OKLCH tokens themselves are
 * untouched). Instantiated once in +layout.svelte and provided via context
 * (constitution VII - no module singletons).
 */
export class ThemeState {
	#current = $state<Theme>(DEFAULT_THEME);

	get current(): Theme {
		return this.#current;
	}

	/**
	 * Reads localStorage["pvmss-theme-v1"]; absent/invalid → prefers-color-scheme. Calls apply().
	 * With no stored preference, also watches prefers-color-scheme so live OS
	 * theme flips re-resolve and re-apply.
	 */
	init(): void {
		this.#current = this.#resolveInitial();
		this.apply();
		if (this.#readStored() === null) this.#watchSystemTheme();
	}

	/** Flips $state, persists, and applies (FR-007/FR-008). */
	toggle(): void {
		this.#current = this.#current === 'dark' ? 'light' : 'dark';
		this.#writeStored(this.#current);
		this.apply();
	}

	/**
	 * Toggles the `dark` class on <html> - same DOM contract as legacy
	 * theme.svelte.ts - and syncs <meta name="theme-color"> to the surface token.
	 */
	apply(): void {
		const dark = this.#current === 'dark';
		document.documentElement.classList.toggle('dark', dark);
		const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
		if (meta !== null) meta.setAttribute('content', dark ? THEME_COLOR_DARK : THEME_COLOR_LIGHT);
	}

	#resolveInitial(): Theme {
		const stored = this.#readStored();
		if (stored !== null) return stored;
		return this.#prefersDark() ? 'dark' : 'light';
	}

	#readStored(): Theme | null {
		const raw = localStorage.getItem(THEME_STORAGE_KEY);
		if (raw !== null && SUPPORTED_THEMES.includes(raw as Theme)) {
			return raw as Theme;
		}
		return null;
	}

	#writeStored(theme: Theme): void {
		try {
			localStorage.setItem(THEME_STORAGE_KEY, theme);
		} catch {
			// Private browsing / locked-down storage: fail closed to the
			// in-memory default (spec Edge Cases) rather than throwing.
		}
	}

	#prefersDark(): boolean {
		return typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches === true;
	}

	/**
	 * Registered by init() only when nothing is stored. The callback re-runs
	 * #resolveInitial(), which reads localStorage again - so once toggle()
	 * persists a choice the listener resolves to the stored value and applying
	 * it is a no-op rather than an override.
	 */
	#watchSystemTheme(): void {
		if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
		window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
			this.#current = this.#resolveInitial();
			this.apply();
		});
	}
}

const THEME_CONTEXT_KEY = Symbol('theme');

/** Called once by the app shell layout. */
export function setThemeContext(): ThemeState {
	const state = new ThemeState();
	setContext(THEME_CONTEXT_KEY, state);
	return state;
}

export function getThemeContext(): ThemeState {
	return getContext<ThemeState>(THEME_CONTEXT_KEY);
}
