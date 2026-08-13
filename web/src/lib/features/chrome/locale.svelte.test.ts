import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LOCALE_STORAGE_KEY, LocaleState } from './locale.svelte';

// T006 (US1): LocaleState — init defaults to "fr" when localStorage is empty
// or holds an unrecognized value; set() persists and calls Paraglide setLocale;
// apply() sets document.documentElement.lang. Tested without a DOM beyond
// happy-dom's document/localStorage.

describe('LocaleState.init', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.lang = '';
	});

	it('defaults to "fr" when localStorage is empty', () => {
		const state = new LocaleState();
		state.init();
		expect(state.current).toBe('fr');
	});

	it('defaults to "fr" when localStorage holds an unrecognized value', () => {
		localStorage.setItem(LOCALE_STORAGE_KEY, 'de');
		const state = new LocaleState();
		state.init();
		expect(state.current).toBe('fr');
	});

	it('restores "en" when localStorage holds a recognized value', () => {
		localStorage.setItem(LOCALE_STORAGE_KEY, 'en');
		const state = new LocaleState();
		state.init();
		expect(state.current).toBe('en');
	});

	it('sets document.documentElement.lang on init', () => {
		const state = new LocaleState();
		state.init();
		expect(document.documentElement.lang).toBe('fr');
	});
});

describe('LocaleState.set', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.lang = '';
	});

	it('persists the chosen locale to localStorage', () => {
		const state = new LocaleState();
		state.init();
		state.set('en');
		expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en');
		expect(state.current).toBe('en');
	});

	it('updates document.documentElement.lang', () => {
		const state = new LocaleState();
		state.init();
		state.set('en');
		expect(document.documentElement.lang).toBe('en');
	});

	it('calls Paraglide setLocale with the new locale', () => {
		const setLocale = vi.fn();
		const state = new LocaleState({ setLocale });
		state.init();
		state.set('en');
		expect(setLocale).toHaveBeenCalledWith('en');
	});
});
