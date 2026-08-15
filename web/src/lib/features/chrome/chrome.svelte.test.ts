import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ChromeState } from './chrome.svelte';

// T006 (US2): ChromeState — owns the sidebar drawer + activity drawer layout
// state. Tested without a DOM beyond happy-dom's window/matchMedia.
//
// Contract (data-model.md "Chrome layout"):
//   - sidebarOpen starts false (drawer closed until the user opens it)
//   - open/close flips it
//   - closeSidebarOnDesktop() forces it false (viewport crossed 900px upward,
//     desktop cannot get stuck "closed")
//   - activityOpen starts false and toggles independently

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

	it('closeSidebarOnDesktop forces the drawer closed', () => {
		const state = new ChromeState();
		state.openSidebar();
		expect(state.sidebarOpen).toBe(true);
		state.closeSidebarOnDesktop();
		expect(state.sidebarOpen).toBe(false);
	});

	it('activity drawer is independent and closed by default', () => {
		const state = new ChromeState();
		expect(state.activityOpen).toBe(false);
		state.openActivity();
		expect(state.activityOpen).toBe(true);
		expect(state.sidebarOpen).toBe(false);
		state.closeActivity();
		expect(state.activityOpen).toBe(false);
	});

	it('toggleActivity flips the activity drawer', () => {
		const state = new ChromeState();
		state.toggleActivity();
		expect(state.activityOpen).toBe(true);
		state.toggleActivity();
		expect(state.activityOpen).toBe(false);
	});
});
