import { describe, expect, it, vi } from 'vitest';
import { ADMIN_NAV_GROUPS } from './admin-nav-items.svelte';
import { SidebarNavigationState } from './sidebar-navigation.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('SidebarNavigationState', () => {
	it.each([
		{ pathname: '/admin', href: '/admin', exact: true, expected: true },
		{ pathname: '/admin/pools', href: '/admin', exact: true, expected: false },
		{ pathname: '/admin/policy/nodes', href: '/admin/policy', exact: true, expected: false },
		{ pathname: '/admin/nodes/default/miniquarium', href: '/admin/nodes', exact: false, expected: true },
		{ pathname: '/vms/default/100', href: '/vms', exact: false, expected: true },
		{ pathname: '/nodes', href: '/', exact: false, expected: false }
	])('matches $pathname against $href with exact=$exact', ({ pathname, href, exact, expected }) => {
		const navigation: SidebarNavigationState = new SidebarNavigationState(1);
		expect(navigation.isItemActive({ pathname, href, exact })).toBe(expected);
	});

	it('keeps the Nodes sidebar link active on node detail routes', () => {
		const nodesItem = ADMIN_NAV_GROUPS.flatMap((group) => group.items).find((item) => item.href === '/admin/nodes');
		expect(nodesItem?.exact).toBe(false);
		if (!nodesItem) throw new Error('Nodes navigation item is missing');
		const navigation: SidebarNavigationState = new SidebarNavigationState(ADMIN_NAV_GROUPS.length);
		expect(navigation.isItemActive({
			pathname: '/admin/nodes/default/miniquarium',
			href: nodesItem.href,
			exact: nodesItem.exact ?? true
		})).toBe(true);
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
