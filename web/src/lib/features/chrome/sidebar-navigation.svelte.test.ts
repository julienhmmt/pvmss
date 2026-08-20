import { describe, expect, it } from 'vitest';
import { SidebarNavigationState } from './sidebar-navigation.svelte';

describe('SidebarNavigationState', () => {
	it.each([
		{ pathname: '/admin', href: '/admin', exact: true, expected: true },
		{ pathname: '/admin/pools', href: '/admin', exact: true, expected: false },
		{ pathname: '/admin/policy/nodes', href: '/admin/policy', exact: true, expected: false },
		{ pathname: '/vms/default/100', href: '/vms', exact: false, expected: true },
		{ pathname: '/nodes', href: '/', exact: false, expected: false }
	])('matches $pathname against $href with exact=$exact', ({ pathname, href, exact, expected }) => {
		const navigation: SidebarNavigationState = new SidebarNavigationState(1);
		expect(navigation.isItemActive({ pathname, href, exact })).toBe(expected);
	});

	it('lets the user close and reopen the active group', () => {
		const navigation: SidebarNavigationState = new SidebarNavigationState(1);
		expect(navigation.isGroupOpen({ index: 0, active: true })).toBe(true);
		navigation.toggleGroup({ index: 0, active: true });
		expect(navigation.isGroupOpen({ index: 0, active: true })).toBe(false);
		navigation.toggleGroup({ index: 0, active: true });
		expect(navigation.isGroupOpen({ index: 0, active: true })).toBe(true);
	});

	it('keeps the explicit user choice when active state changes', () => {
		const navigation: SidebarNavigationState = new SidebarNavigationState(1);
		navigation.toggleGroup({ index: 0, active: false });
		expect(navigation.isGroupOpen({ index: 0, active: true })).toBe(true);
		navigation.toggleGroup({ index: 0, active: true });
		expect(navigation.isGroupOpen({ index: 0, active: false })).toBe(false);
	});
});
