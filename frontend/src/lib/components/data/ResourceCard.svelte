<script lang="ts">
	import type { Component } from 'svelte';
	import * as Card from '$lib/components/ui/card';

	interface Props {
		title: string;
		value: string | number;
		icon?: Component;
		subtitle?: string;
	}

	let { title, value, icon: Icon, subtitle }: Props = $props();
</script>

<Card.Root>
	<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
		<Card.Title class="text-sm font-medium">{title}</Card.Title>
		{#if Icon}
			<Icon class="h-4 w-4 text-muted-foreground" />
		{/if}
	</Card.Header>
	<Card.Content>
		<div class="text-2xl font-bold">{value}</div>
		{#if subtitle}
			<p class="text-xs text-muted-foreground">{subtitle}</p>
		{/if}
	</Card.Content>
</Card.Root>
