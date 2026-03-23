<script lang="ts">
	import type { Snippet } from 'svelte';
	import { SidebarProvider, SidebarInset, SidebarTrigger } from '$lib/components/ui/sidebar';

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
		</header>
		<main class="flex-1 overflow-auto p-6">
			{@render children()}
		</main>
	</SidebarInset>
</SidebarProvider>
