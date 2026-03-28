<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Component } from 'svelte';

	interface Props {
		title: string;
		description?: string;
		icon?: Component;
		actions?: Snippet;
	}

	let { title, description, icon: Icon, actions }: Props = $props();
</script>

<div class="flex items-center justify-between pb-6">
	<div class="flex items-center gap-3">
		{#if Icon}
			<Icon class="h-7 w-7 text-muted-foreground" />
		{/if}
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{title}</h1>
			{#if description}
				<p class="text-sm text-muted-foreground">{description}</p>
			{/if}
		</div>
	</div>
	{#if actions}
		<div class="flex items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</div>
