<script lang="ts">
	import type { Snippet, Component } from 'svelte';

	interface Props {
		title: string;
		description?: string;
		icon?: Component;
		action?: Snippet;
	}

	let { title, description, icon: Icon, action }: Props = $props();
</script>

<div class="flex flex-col items-center justify-center py-16 text-center">
	{#if Icon}
		<Icon class="mb-4 h-12 w-12 text-muted-foreground/50" />
	{/if}
	<h3 class="text-lg font-medium">{title}</h3>
	{#if description}
		<p class="mt-1 max-w-sm text-sm text-muted-foreground">{description}</p>
	{/if}
	{#if action}
		<div class="mt-4">
			{@render action()}
		</div>
	{/if}
</div>
