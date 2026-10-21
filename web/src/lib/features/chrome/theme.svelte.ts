import { getContext, setContext } from 'svelte';

export type Theme = 'light' | 'dark';
/** What the user picked; `system` follows prefers-color-scheme live. */
export type ThemePreference = Theme | 'system';

const PINNED_THEMES: readonly Theme[] = ['light', 'dark'];

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
	#preference = $state<ThemePreference>('system');
	#current = $state<Theme>('light');

	/** The user's choice, including `system`. */
	get preference(): ThemePreference {
		return this.#preference;
	}

	/** The theme actually shown. */
	get current(): Theme {
		return this.#current;
	}

	/**
	 * Reads localStorage["pvmss-theme-v1"] (light/dark; absent or invalid means
	 * system), applies it, and watches prefers-color-scheme so `system` follows
	 * live OS flips.
	 */
	init(): void {
		this.#preference = this.#readStored() ?? 'system';
		this.#resolve();
		this.#watchSystemTheme();
	}

	/** Persists the choice (system clears the key) and applies it. */
	set(preference: ThemePreference): void {
		this.#preference = preference;
		this.#writeStored(preference);
		this.#resolve();
	}

	/**
	 * Toggles the `dark` class on <html> - same DOM contract as the inline
	 * bootstrap in app.html - and syncs <meta name="theme-color"> to the surface token.
	 */
	apply(): void {
		const dark = this.#current === 'dark';
		document.documentElement.classList.toggle('dark', dark);
		const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
		if (meta !== null) meta.setAttribute('content', dark ? THEME_COLOR_DARK : THEME_COLOR_LIGHT);
	}

	#resolve(): void {
		if (this.#preference === 'system') {
			this.#current = this.#prefersDark() ? 'dark' : 'light';
		} else {
			this.#current = this.#preference;
		}
		this.apply();
	}

	#readStored(): Theme | null {
		try {
			const raw = localStorage.getItem(THEME_STORAGE_KEY);
			return raw !== null && PINNED_THEMES.includes(raw as Theme) ? (raw as Theme) : null;
		} catch {
			return null;
		}
	}

	#writeStored(preference: ThemePreference): void {
		try {
			if (preference === 'system') localStorage.removeItem(THEME_STORAGE_KEY);
			else localStorage.setItem(THEME_STORAGE_KEY, preference);
		} catch {
			// Private browsing / locked-down storage: the choice holds for this
			// page only (spec Edge Cases) rather than throwing.
		}
	}

	#prefersDark(): boolean {
		return typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches === true;
	}

	/** OS flips only matter while the preference is `system`. */
	#watchSystemTheme(): void {
		if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
		window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
			if (this.#preference === 'system') this.#resolve();
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
