<script lang="ts">
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import { t } from 'svelte-i18n';
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

	const items = $derived([
		{ href: `${base}/`, icon: House, label: $t('nav.dashboard') },
		{ href: `${base}/nodes`, icon: HardDrives, label: $t('nav.nodes') },
		{ href: `${base}/storage`, icon: Database, label: $t('nav.storage') },
		{ href: `${base}/vms`, icon: Desktop, label: $t('nav.vms') },
		{ href: `${base}/userpool`, icon: UsersThree, label: $t('nav.userpool') },
		{ href: `${base}/tags`, icon: Tag, label: $t('nav.tags') },
		{ href: `${base}/limits`, icon: Sliders, label: $t('nav.limits') },
		{ href: `${base}/vmbr`, icon: WifiHigh, label: $t('nav.network') },
		{ href: `${base}/cloudinit`, icon: Cloud, label: $t('nav.cloudinit') },
		{ href: `${base}/iso`, icon: Disc, label: $t('nav.iso') },
		{ href: `${base}/appinfo`, icon: Info, label: $t('nav.appinfo') }
	]);

	function isActive(itemHref: string, currentPath: string): boolean {
		if (itemHref === `${base}/`) return currentPath === `${base}` || currentPath === `${base}/`;
		return currentPath.startsWith(itemHref);
	}
</script>

<Sidebar>
	<SidebarHeader>
		<div class="flex items-center gap-2 px-2 py-1">
			<span class="text-lg font-bold">PVMSS</span>
			<span class="text-xs text-muted-foreground">{$t('common.admin')}</span>
		</div>
	</SidebarHeader>
	<SidebarContent>
		<SidebarGroup>
			<SidebarGroupLabel>{$t('nav.administration')}</SidebarGroupLabel>
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
			{$t('nav.sidebarFooter')}
		</div>
	</SidebarFooter>
</Sidebar>
