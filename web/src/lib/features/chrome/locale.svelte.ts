import { getContext, setContext } from 'svelte';
import { setLocale as paraglideSetLocale, type Locale } from '$lib/paraglide/runtime.js';

/**
 * Supported interface locales. Adding a third locale is one more message file
 * plus one entry here — never a change to the switcher component (FR-003).
 */
const SUPPORTED_LOCALES: readonly Locale[] = ['fr', 'en'];
const DEFAULT_LOCALE: Locale = 'fr';

/** localStorage key for the persisted locale preference (FR-005). */
export const LOCALE_STORAGE_KEY = 'pvmss-locale';

/** Paraglide setLocale injection seam — overridable in tests. */
export interface LocaleRuntime {
	setLocale: (locale: Locale) => void;
}

const defaultRuntime: LocaleRuntime = { setLocale: paraglideSetLocale };

/**
 * LocaleState owns the active interface language: a $state-backed current
 * locale, persisted to localStorage, kept in sync with Paraglide's runtime and
 * document.documentElement.lang (FR-004/FR-005). Instantiated once in
 * +layout.svelte and provided via context (constitution VII — no module
 * singletons).
 */
export class LocaleState {
	#current = $state<Locale>(DEFAULT_LOCALE);
	#runtime: LocaleRuntime;

	constructor(runtime: LocaleRuntime = defaultRuntime) {
		this.#runtime = runtime;
	}

	get current(): Locale {
		return this.#current;
	}

	/** Reads localStorage; absent or unrecognized → "fr". Calls apply(). */
	init(): void {
		const stored = this.#readStored();
		this.#current = stored;
		this.#runtime.setLocale(stored);
		this.apply();
	}

	/** Writes $state, localStorage, Paraglide setLocale, and applies lang. */
	set(locale: Locale): void {
		this.#current = locale;
		this.#writeStored(locale);
		this.#runtime.setLocale(locale);
		this.apply();
	}

	/** Sets document.documentElement.lang to the active locale (FR-004). */
	apply(): void {
		document.documentElement.lang = this.#current;
	}

	#readStored(): Locale {
		const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
		if (stored !== null && this.#isSupported(stored)) {
			return stored as Locale;
		}
		return DEFAULT_LOCALE;
	}

	#writeStored(locale: Locale): void {
		try {
			localStorage.setItem(LOCALE_STORAGE_KEY, locale);
		} catch {
			// Private browsing / locked-down storage: fail closed to the
			// in-memory default (spec Edge Cases) rather than throwing.
		}
	}

	#isSupported(value: string): boolean {
		return SUPPORTED_LOCALES.includes(value as Locale);
	}
}

const LOCALE_CONTEXT_KEY = Symbol('locale');

/** Called once by the app shell layout. */
export function setLocaleContext(runtime?: LocaleRuntime): LocaleState {
	const state = new LocaleState(runtime);
	setContext(LOCALE_CONTEXT_KEY, state);
	return state;
}

export function getLocaleContext(): LocaleState {
	return getContext<LocaleState>(LOCALE_CONTEXT_KEY);
}
