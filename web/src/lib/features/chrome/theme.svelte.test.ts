import { beforeEach, describe, expect, it, vi } from 'vitest';
import { THEME_COLOR_DARK, THEME_COLOR_LIGHT, THEME_STORAGE_KEY, ThemeState } from './theme.svelte';

// T008 (US2): ThemeState - init reads localStorage["pvmss-theme-v1"] when
// present, else falls back to prefers-color-scheme; toggle() flips, persists,
// and calls apply(), which toggles the "dark" class. Tested without a DOM
// beyond happy-dom's document/localStorage/matchMedia.

interface MatchMediaStub {
	osPrefersDark: { value: boolean };
	changeListeners: (() => void)[];
}

// matchMedia stub whose `matches` reads a mutable flag, so a test can flip the
// OS preference afterwards and fire the captured `change` listeners itself
// (happy-dom does not emit MediaQueryList change events).
function stubMatchMedia(initialPrefersDark: boolean): MatchMediaStub {
	const osPrefersDark = { value: initialPrefersDark };
	const changeListeners: (() => void)[] = [];
	vi.stubGlobal(
		'matchMedia',
		vi.fn().mockImplementation((query: string) => ({
			get matches(): boolean {
				return query.includes('dark') ? osPrefersDark.value : false;
			},
			media: query,
			onchange: null,
			addEventListener: vi.fn((type: string, listener: () => void) => {
				if (type === 'change') changeListeners.push(listener);
			}),
			removeEventListener: vi.fn(),
			addListener: vi.fn(),
			removeListener: vi.fn(),
			dispatchEvent: vi.fn()
		}))
	);
	return { osPrefersDark, changeListeners };
}

function setPrefersDark(prefers: boolean): void {
	stubMatchMedia(prefers);
}

// The meta element lives in app.html, which the test DOM does not load.
function insertThemeColorMeta(): HTMLMetaElement {
	const meta = document.createElement('meta');
	meta.setAttribute('name', 'theme-color');
	document.head.appendChild(meta);
	return meta;
}

describe('ThemeState.init', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.classList.remove('dark');
	});

	it('uses the stored value when present and valid', () => {
		localStorage.setItem(THEME_STORAGE_KEY, 'dark');
		const state = new ThemeState();
		state.init();
		expect(state.current).toBe('dark');
		expect(document.documentElement.classList.contains('dark')).toBe(true);
	});

	it('falls back to prefers-color-scheme: dark when no stored value', () => {
		setPrefersDark(true);
		const state = new ThemeState();
		state.init();
		expect(state.current).toBe('dark');
		expect(document.documentElement.classList.contains('dark')).toBe(true);
	});

	it('falls back to light when no stored value and prefers-color-scheme is light', () => {
		setPrefersDark(false);
		const state = new ThemeState();
		state.init();
		expect(state.current).toBe('light');
		expect(document.documentElement.classList.contains('dark')).toBe(false);
	});

	it('discards an unrecognized stored value and falls back to prefers-color-scheme', () => {
		localStorage.setItem(THEME_STORAGE_KEY, 'hot-pink');
		setPrefersDark(false);
		const state = new ThemeState();
		state.init();
		expect(state.current).toBe('light');
	});
});

describe('ThemeState.toggle', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.classList.remove('dark');
		setPrefersDark(false);
	});

	it('flips light to dark, persists, and applies the dark class', () => {
		const state = new ThemeState();
		state.init();
		expect(state.current).toBe('light');
		state.toggle();
		expect(state.current).toBe('dark');
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
		expect(document.documentElement.classList.contains('dark')).toBe(true);
	});

	it('flips dark back to light and removes the dark class', () => {
		localStorage.setItem(THEME_STORAGE_KEY, 'dark');
		const state = new ThemeState();
		state.init();
		state.toggle();
		expect(state.current).toBe('light');
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
		expect(document.documentElement.classList.contains('dark')).toBe(false);
	});
});

describe('ThemeState theme-color meta', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.classList.remove('dark');
		document.head.innerHTML = '';
	});

	it.each([
		{ scenario: 'stored light', stored: 'light', prefersDark: true, expected: THEME_COLOR_LIGHT },
		{ scenario: 'stored dark', stored: 'dark', prefersDark: false, expected: THEME_COLOR_DARK },
		{ scenario: 'OS dark, nothing stored', stored: null, prefersDark: true, expected: THEME_COLOR_DARK },
		{ scenario: 'OS light, nothing stored', stored: null, prefersDark: false, expected: THEME_COLOR_LIGHT }
	])('sets meta[name=theme-color] to $expected on init ($scenario)', ({ stored, prefersDark, expected }) => {
		if (stored !== null) localStorage.setItem(THEME_STORAGE_KEY, stored);
		stubMatchMedia(prefersDark);
		const meta = insertThemeColorMeta();
		new ThemeState().init();
		expect(meta.getAttribute('content')).toBe(expected);
	});

	it('updates meta[name=theme-color] on toggle', () => {
		setPrefersDark(false);
		const meta = insertThemeColorMeta();
		const state = new ThemeState();
		state.init();
		expect(meta.getAttribute('content')).toBe(THEME_COLOR_LIGHT);
		state.toggle();
		expect(meta.getAttribute('content')).toBe(THEME_COLOR_DARK);
	});

	it('does not throw when the meta element is absent', () => {
		setPrefersDark(true);
		expect(() => new ThemeState().init()).not.toThrow();
	});
});

describe('ThemeState OS theme listener', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.classList.remove('dark');
		document.head.innerHTML = '';
	});

	it.each([
		{ scenario: 'stored light', stored: 'light', prefersDark: true, expectedListeners: 0 },
		{ scenario: 'stored dark', stored: 'dark', prefersDark: false, expectedListeners: 0 },
		{ scenario: 'nothing stored, OS light', stored: null, prefersDark: false, expectedListeners: 1 },
		{ scenario: 'nothing stored, OS dark', stored: null, prefersDark: true, expectedListeners: 1 }
	])(
		'registers $expectedListeners change listener(s) on init ($scenario)',
		({ stored, prefersDark, expectedListeners }) => {
			if (stored !== null) localStorage.setItem(THEME_STORAGE_KEY, stored);
			const { changeListeners } = stubMatchMedia(prefersDark);
			new ThemeState().init();
			expect(changeListeners).toHaveLength(expectedListeners);
		}
	);

	it('re-resolves and re-applies when the OS theme flips', () => {
		const { osPrefersDark, changeListeners } = stubMatchMedia(false);
		const meta = insertThemeColorMeta();
		const state = new ThemeState();
		state.init();
		expect(state.current).toBe('light');
		osPrefersDark.value = true;
		for (const listener of changeListeners) listener();
		expect(state.current).toBe('dark');
		expect(document.documentElement.classList.contains('dark')).toBe(true);
		expect(meta.getAttribute('content')).toBe(THEME_COLOR_DARK);
	});

	it('leaves a stored preference untouched when the OS theme flips', () => {
		const { changeListeners } = stubMatchMedia(true);
		const meta = insertThemeColorMeta();
		const state = new ThemeState();
		state.init();
		state.toggle();
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
		for (const listener of changeListeners) listener();
		expect(state.current).toBe('light');
		expect(document.documentElement.classList.contains('dark')).toBe(false);
		expect(meta.getAttribute('content')).toBe(THEME_COLOR_LIGHT);
	});
});
