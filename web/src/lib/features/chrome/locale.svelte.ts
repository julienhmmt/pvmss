import { getContext, setContext } from 'svelte';
import { locales, setLocale as paraglideSetLocale, type Locale } from '#lib/paraglide/runtime.js';

/**
 * Supported interface locales come from project.inlang/settings.json. Adding
 * a locale is one message file, one entry there and its name below.
 */
export const SUPPORTED_LOCALES: readonly Locale[] = locales;
/** Fallback when neither a stored choice nor the browser language matches. */
const DEFAULT_LOCALE: Locale = 'en';

/** Each locale named in its own language, as language pickers show them. */
export const LOCALE_NAMES: Record<Locale, string> = {
	fr: 'Français',
	en: 'English',
	de: 'Deutsch',
	it: 'Italiano'
};

/** Three-letter label shown on the switcher button. Locale codes stay en/fr/de/it. */
export const LOCALE_SHORT_LABELS: Record<Locale, string> = {
	fr: 'FRA',
	en: 'ENG',
	de: 'DEU',
	it: 'ITA'
};

/** localStorage key for the persisted locale preference (FR-005). */
export const LOCALE_STORAGE_KEY = 'pvmss-locale';

/** Paraglide setLocale injection seam - overridable in tests. */
export interface LocaleRuntime {
	setLocale: (locale: Locale) => void;
}

const defaultRuntime: LocaleRuntime = {
	setLocale: (locale: Locale): void => {
		void paraglideSetLocale(locale);
	}
};

/**
 * LocaleState owns the active interface language: a $state-backed current
 * locale, persisted to localStorage, kept in sync with Paraglide's runtime and
 * document.documentElement.lang (FR-004/FR-005). Instantiated once in
 * +layout.svelte and provided via context (constitution VII - no module
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

	/** Reads localStorage, then the browser languages, then "en". Calls apply(). */
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
		return browserLocale() ?? DEFAULT_LOCALE;
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

/** Documentation pages exist in en and fr only; other locales read English. */
export function docLocale(locale: Locale): 'en' | 'fr' {
	return locale === 'fr' ? 'fr' : 'en';
}

/** First browser language whose primary subtag we support ("de-CH" -> "de"). */
function browserLocale(): Locale | undefined {
	const preferred = navigator.languages.length > 0 ? navigator.languages : [navigator.language];
	for (const tag of preferred) {
		const primary = tag.split('-')[0]?.toLowerCase() ?? '';
		if (SUPPORTED_LOCALES.includes(primary as Locale)) return primary as Locale;
	}
	return undefined;
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
