import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ChromeState } from './chrome.svelte';

// T006 (US2): ChromeState — owns the sidebar drawer layout state. Tested
// without a DOM beyond happy-dom's window/matchMedia.
//
// Contract (data-model.md "Chrome layout"):
//   - sidebarOpen starts false (drawer closed until the user opens it)
//   - open/close flips it
//   - closeSidebar() forces it false (viewport crossed 900px upward,
//     desktop cannot get stuck "closed")

function stubViewport(width: number): void {
	vi.stubGlobal('matchMedia', (query: string) => ({
		matches: query.includes('max-width') ? width <= 900 : width >= 900,
		media: query,
		onchange: null,
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		addListener: vi.fn(),
		removeListener: vi.fn(),
		dispatchEvent: vi.fn()
	}));
}

describe('ChromeState', () => {
	beforeEach(() => {
		stubViewport(1200);
	});

	it('drawer is closed by default', () => {
		const state = new ChromeState();
		expect(state.sidebarOpen).toBe(false);
	});

	it('openSidebar / closeSidebar flip the drawer', () => {
		const state = new ChromeState();
		state.openSidebar();
		expect(state.sidebarOpen).toBe(true);
		state.closeSidebar();
		expect(state.sidebarOpen).toBe(false);
	});

	it('closeSidebar forces the drawer closed', () => {
		const state = new ChromeState();
		state.openSidebar();
		expect(state.sidebarOpen).toBe(true);
		state.closeSidebar();
		expect(state.sidebarOpen).toBe(false);
	});
});
