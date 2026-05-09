<script lang="ts">
	import type { Snippet, Component } from 'svelte';
	import { cn } from '$lib/utils.js';

	interface Props {
		icon?: Component;
		title?: string;
		description?: string;
		actions?: Snippet;
		class?: string;
		compact?: boolean;
	}

	let {
		icon: Icon,
		title,
		description,
		actions,
		class: className,
		compact = false
	}: Props = $props();
</script>

<div
	class={cn(
		'flex flex-col items-center justify-center text-center',
		compact ? 'py-6 gap-2' : 'py-12 gap-3',
		className
	)}
>
	{#if Icon}
		<Icon class={cn('text-muted-foreground/50', compact ? 'h-6 w-6' : 'h-10 w-10')} />
	{/if}
	{#if title}
		<p class={cn('font-medium text-foreground', compact ? 'text-sm' : 'text-base')}>{title}</p>
	{/if}
	{#if description}
		<p class="text-sm text-muted-foreground max-w-md">{description}</p>
	{/if}
	{#if actions}
		<div class="flex items-center gap-2 mt-2">{@render actions()}</div>
	{/if}
</div>
