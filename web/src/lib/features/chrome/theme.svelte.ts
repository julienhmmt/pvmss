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
 * ThemeState owns the light/dark preference: a $state-backed current theme,
 * persisted under a versioned localStorage key, applied by toggling the
 * `dark` class on <html> (constitution X: the OKLCH tokens themselves are
 * untouched). Instantiated once in +layout.svelte and provided via context
 * (constitution VII — no module singletons).
 */
export class ThemeState {
	#current = $state<Theme>(DEFAULT_THEME);

	get current(): Theme {
		return this.#current;
	}

	/** Reads localStorage["pvmss-theme-v1"]; absent/invalid → prefers-color-scheme. Calls apply(). */
	init(): void {
		this.#current = this.#resolveInitial();
		this.apply();
	}

	/** Flips $state, persists, and applies (FR-007/FR-008). */
	toggle(): void {
		this.#current = this.#current === 'dark' ? 'light' : 'dark';
		this.#writeStored(this.#current);
		this.apply();
	}

	/** Toggles the `dark` class on <html> — same DOM contract as legacy theme.svelte.ts. */
	apply(): void {
		document.documentElement.classList.toggle('dark', this.#current === 'dark');
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
