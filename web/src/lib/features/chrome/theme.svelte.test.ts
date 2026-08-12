import { beforeEach, describe, expect, it, vi } from 'vitest';
import { THEME_STORAGE_KEY, ThemeState } from './theme.svelte';

// T008 (US2): ThemeState — init reads localStorage["pvmss-theme-v1"] when
// present, else falls back to prefers-color-scheme; toggle() flips, persists,
// and calls apply(), which toggles the "dark" class. Tested without a DOM
// beyond happy-dom's document/localStorage/matchMedia.

function setPrefersDark(prefers: boolean): void {
	vi.stubGlobal(
		'matchMedia',
		vi.fn().mockImplementation((query: string) => ({
			matches: query.includes('dark') ? prefers : false,
			media: query,
			onchange: null,
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			addListener: vi.fn(),
			removeListener: vi.fn(),
			dispatchEvent: vi.fn()
		}))
	);
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
