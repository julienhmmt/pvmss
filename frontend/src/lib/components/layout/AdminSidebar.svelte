<script lang="ts">
	import { page } from '$app/stores';
	import { t } from 'svelte-i18n';
	import {
		Sidebar,
		SidebarContent,
		SidebarHeader,
		SidebarFooter,
		SidebarMenu,
		SidebarMenuItem
	} from '$lib/components/ui/sidebar';
	import {
		House,
		HardDrives,
		Database,
		Desktop,
		UsersThree,
		Tag,
		Sliders,
		WifiHigh,
		Cloud,
		Disc,
		Cube,
		Gear
	} from 'phosphor-svelte';

	const items = $derived([
		{ href: `/admin/`, icon: House, label: $t('nav.dashboard') },
		{ href: `/admin/nodes`, icon: HardDrives, label: $t('nav.nodes') },
		{ href: `/admin/storage`, icon: Database, label: $t('nav.storage') },
		{ href: `/admin/vms`, icon: Desktop, label: $t('nav.vms') },
		{ href: `/admin/userpool`, icon: UsersThree, label: $t('nav.userpool') },
		{ href: `/admin/tags`, icon: Tag, label: $t('nav.tags') },
		{ href: `/admin/limits`, icon: Sliders, label: $t('nav.limits') },
		{ href: `/admin/vmbr`, icon: WifiHigh, label: $t('nav.network') },
		{ href: `/admin/cloudinit`, icon: Cloud, label: $t('nav.cloudinit') },
		{ href: `/admin/iso`, icon: Disc, label: $t('nav.iso') },
		{ href: `/admin/profiles`, icon: Cube, label: $t('nav.profiles') },
		{ href: `/admin/settings`, icon: Gear, label: $t('nav.settings') }
	]);

	function isActive(itemHref: string, currentPath: string): boolean {
		if (itemHref === `/admin/`) return currentPath === '/admin' || currentPath === '/admin/';
		return currentPath.startsWith(itemHref);
	}
</script>

<Sidebar>
	<!-- Header -->
	<SidebarHeader class="px-4 py-4 border-b border-border">
		<div class="flex items-center gap-3">
			<div class="pv-sidebar-logo">PV</div>
			<div>
				<div class="text-base font-bold tracking-tight leading-none">PVMSS</div>
				<div class="text-[0.68rem] font-semibold uppercase tracking-widest text-muted-foreground mt-0.5">
					{$t('common.admin')}
				</div>
			</div>
		</div>
	</SidebarHeader>

	<!-- Nav -->
	<SidebarContent class="px-3 py-3">
		<div class="pv-sidebar-section-label">{$t('nav.administration')}</div>
		<SidebarMenu class="gap-0.5">
			{#each items as item}
				{@const active = isActive(item.href, $page.url.pathname)}
				<SidebarMenuItem>
					<a href={item.href} class="pv-sidebar-item {active ? 'pv-sidebar-item--active' : ''}">
						<span class="pv-sidebar-icon-wrap {active ? 'pv-sidebar-icon-wrap--active' : ''}">
							<item.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4" />
						</span>
						<span class="pv-sidebar-item-label">{item.label}</span>
					</a>
				</SidebarMenuItem>
			{/each}
		</SidebarMenu>
	</SidebarContent>

	<!-- Footer -->
	<SidebarFooter class="px-4 py-3 border-t border-border">
		<div class="text-[0.68rem] text-muted-foreground font-medium">
			{$t('nav.sidebarFooter')}
		</div>
	</SidebarFooter>
</Sidebar>

<style>
	:global(.pv-sidebar-logo) {
		width: 36px;
		height: 36px;
		border-radius: 10px;
		background: linear-gradient(135deg, hsl(var(--blaze-orange-600)), hsl(var(--blaze-orange-800)));
		color: white;
		font-size: 0.78rem;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		letter-spacing: 0.02em;
	}

	:global(.pv-sidebar-section-label) {
		font-size: 0.65rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--muted-foreground);
		padding: 0 8px;
		margin-bottom: 6px;
		margin-top: 2px;
	}

	:global(.pv-sidebar-item) {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 7px 8px;
		border-radius: 9px;
		text-decoration: none;
		color: var(--sidebar-foreground);
		font-size: 0.875rem;
		font-weight: 500;
		transition: background 0.12s, color 0.12s;
		width: 100%;
	}

	:global(.pv-sidebar-item:hover:not(.pv-sidebar-item--active)) {
		background: var(--sidebar-accent);
		color: var(--sidebar-accent-foreground);
	}

	:global(.pv-sidebar-item--active) {
		background: hsl(var(--blaze-orange-600));
		color: white;
		font-weight: 600;
	}

	:global(.dark .pv-sidebar-item--active) {
		background: hsl(var(--blaze-orange-700));
	}

	:global(.pv-sidebar-icon-wrap) {
		width: 28px;
		height: 28px;
		border-radius: 7px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: transparent;
		color: var(--muted-foreground);
		transition: background 0.12s, color 0.12s;
	}

	:global(.pv-sidebar-item:hover:not(.pv-sidebar-item--active) .pv-sidebar-icon-wrap) {
		background: var(--sidebar-accent);
		color: var(--sidebar-accent-foreground);
	}

	:global(.pv-sidebar-icon-wrap--active) {
		background: hsl(var(--blaze-orange-500) / 0.3);
		color: white;
	}

	:global(.pv-sidebar-item-label) {
		flex: 1;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
