<script lang="ts">
	import type { Snippet } from 'svelte';
	import { SidebarProvider, SidebarInset, SidebarTrigger } from '$lib/components/ui/sidebar';
	import { Separator } from '$lib/components/ui/separator';

	interface Props {
		sidebar: Snippet;
		children: Snippet;
	}

	let { sidebar, children }: Props = $props();
</script>

<SidebarProvider>
	{@render sidebar()}
	<SidebarInset>
		<header class="flex h-10 shrink-0 items-center gap-2 border-b px-4">
			<SidebarTrigger class="-ml-1" />
			<Separator orientation="vertical" class="mr-2 h-4" />
			<span class="text-muted-foreground text-xs">Administration</span>
		</header>
		<main class="flex-1 overflow-auto p-6">
			{@render children()}
		</main>
	</SidebarInset>
</SidebarProvider>
