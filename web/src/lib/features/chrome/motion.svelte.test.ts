import { afterEach, describe, expect, it } from 'vitest';
import { ChromeState, MOTION_STORAGE_KEY } from './chrome.svelte';

describe('ChromeState motion toggle', () => {
	afterEach(() => {
		localStorage.clear();
		document.documentElement.removeAttribute('data-motion-off');
	});

	it('stops motion, remembers it, and plays again', () => {
		const chrome = new ChromeState();
		expect(chrome.motionOff).toBe(false);

		chrome.toggleMotion();
		expect(document.documentElement.hasAttribute('data-motion-off')).toBe(true);
		expect(localStorage.getItem(MOTION_STORAGE_KEY)).toBe('off');
		expect(new ChromeState().motionOff).toBe(true);

		chrome.toggleMotion();
		expect(document.documentElement.hasAttribute('data-motion-off')).toBe(false);
		expect(localStorage.getItem(MOTION_STORAGE_KEY)).toBeNull();
	});
});
