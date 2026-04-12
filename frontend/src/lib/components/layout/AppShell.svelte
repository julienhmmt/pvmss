<script lang="ts">
	import type { Snippet } from 'svelte';
	import { SidebarProvider, SidebarInset } from '$lib/components/ui/sidebar';

	interface Props {
		sidebar: Snippet;
		children: Snippet;
	}

	let { sidebar, children }: Props = $props();
</script>

<SidebarProvider>
	{@render sidebar()}
	<SidebarInset class="flex flex-col">
		<main class="flex-1 overflow-auto p-6">
			{@render children()}
		</main>
	</SidebarInset>
</SidebarProvider>
