import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { m } from '#lib/paraglide/messages.js';
import { THEME_STORAGE_KEY } from './theme.svelte';
import ThemeToggleHarness from './ThemeToggleHarness.test.svelte';

function button(name: string): HTMLButtonElement {
	const found = [...document.querySelectorAll<HTMLButtonElement>('button')].find((el) => el.getAttribute('aria-label') === name);
	if (!found) throw new Error(`no button ${name}`);
	return found;
}

describe('ThemeToggle', () => {
	beforeEach(() => {
		localStorage.clear();
		vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: false, addEventListener: vi.fn() }));
	});
	afterEach(() => {
		document.body.innerHTML = '';
		vi.unstubAllGlobals();
	});

	it('offers light, dark and system, with system pressed by default', async () => {
		const app = mount(ThemeToggleHarness, { target: document.body });
		flushSync();

		expect(document.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe(m['nav.theme']());
		expect(button(m['nav.theme.system']()).getAttribute('aria-pressed')).toBe('true');
		expect(button(m['nav.theme.dark']()).getAttribute('aria-pressed')).toBe('false');
		await unmount(app);
	});

	it('pins the clicked theme', async () => {
		const app = mount(ThemeToggleHarness, { target: document.body });
		button(m['nav.theme.dark']()).click();
		flushSync();

		expect(button(m['nav.theme.dark']()).getAttribute('aria-pressed')).toBe('true');
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
		expect(document.documentElement.classList.contains('dark')).toBe(true);
		await unmount(app);
	});
});
