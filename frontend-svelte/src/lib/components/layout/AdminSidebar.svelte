<script lang="ts">
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import {
		Sidebar,
		SidebarContent,
		SidebarGroup,
		SidebarGroupContent,
		SidebarGroupLabel,
		SidebarMenu,
		SidebarMenuItem,
		SidebarMenuButton,
		SidebarHeader,
		SidebarFooter
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
		Info
	} from 'phosphor-svelte';

	const items = [
		{ href: `${base}/`, icon: House, label: 'Dashboard' },
		{ href: `${base}/nodes`, icon: HardDrives, label: 'Nodes' },
		{ href: `${base}/storage`, icon: Database, label: 'Storage' },
		{ href: `${base}/vms`, icon: Desktop, label: 'Virtual Machines' },
		{ href: `${base}/userpool`, icon: UsersThree, label: 'User Pools' },
		{ href: `${base}/tags`, icon: Tag, label: 'Tags' },
		{ href: `${base}/limits`, icon: Sliders, label: 'Limits' },
		{ href: `${base}/vmbr`, icon: WifiHigh, label: 'Network' },
		{ href: `${base}/cloudinit`, icon: Cloud, label: 'Cloud-Init' },
		{ href: `${base}/iso`, icon: Disc, label: 'ISO Images' },
		{ href: `${base}/appinfo`, icon: Info, label: 'App Info' }
	];

	function isActive(itemHref: string, currentPath: string): boolean {
		if (itemHref === `${base}/`) return currentPath === `${base}` || currentPath === `${base}/`;
		return currentPath.startsWith(itemHref);
	}
</script>

<Sidebar>
	<SidebarHeader>
		<div class="flex items-center gap-2 px-2 py-1">
			<span class="text-lg font-bold">PVMSS</span>
			<span class="text-xs text-muted-foreground">Admin</span>
		</div>
	</SidebarHeader>
	<SidebarContent>
		<SidebarGroup>
			<SidebarGroupLabel>Administration</SidebarGroupLabel>
			<SidebarGroupContent>
				<SidebarMenu>
					{#each items as item}
						<SidebarMenuItem>
							<SidebarMenuButton
								isActive={isActive(item.href, $page.url.pathname)}
							>
								{#snippet child({ props })}
									<a href={item.href} {...props}>
										<item.icon class="h-4 w-4" />
										<span>{item.label}</span>
									</a>
								{/snippet}
							</SidebarMenuButton>
						</SidebarMenuItem>
					{/each}
				</SidebarMenu>
			</SidebarGroupContent>
		</SidebarGroup>
	</SidebarContent>
	<SidebarFooter>
		<div class="px-2 py-1 text-xs text-muted-foreground">
			PVMSS Admin Panel
		</div>
	</SidebarFooter>
</Sidebar>
