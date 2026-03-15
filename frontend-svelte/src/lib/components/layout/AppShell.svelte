<script lang="ts">
	import type { Snippet } from 'svelte';
	import { SidebarProvider, SidebarInset, SidebarTrigger } from '$lib/components/ui/sidebar';
	import { Separator } from '$lib/components/ui/separator';
	import ThemeToggle from './ThemeToggle.svelte';
	import { auth } from '$lib/stores/auth.svelte';

	interface Props {
		sidebar: Snippet;
		children: Snippet;
	}

	let { sidebar, children }: Props = $props();
</script>

<SidebarProvider>
	{@render sidebar()}
	<SidebarInset>
		<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<SidebarTrigger class="-ml-1" />
			<Separator orientation="vertical" class="mr-2 h-4" />
			<div class="flex-1" />
			<span class="text-sm text-muted-foreground">{auth.username}</span>
			<ThemeToggle />
		</header>
		<main class="flex-1 overflow-auto p-6">
			{@render children()}
		</main>
	</SidebarInset>
</SidebarProvider>
