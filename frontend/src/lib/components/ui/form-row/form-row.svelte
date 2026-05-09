<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Label } from '$lib/components/ui/label';
	import { cn } from '$lib/utils.js';

	interface Props {
		label?: string;
		htmlFor?: string;
		hint?: string;
		error?: string | null;
		required?: boolean;
		class?: string;
		children: Snippet;
	}

	let {
		label,
		htmlFor,
		hint,
		error,
		required = false,
		class: className,
		children
	}: Props = $props();
</script>

<div class={cn('flex flex-col gap-1.5', className)}>
	{#if label}
		<Label for={htmlFor}>
			{label}
			{#if required}<span class="text-destructive ml-0.5" aria-hidden="true">*</span>{/if}
		</Label>
	{/if}
	{@render children()}
	{#if error}
		<p role="alert" class="text-xs text-destructive">{error}</p>
	{:else if hint}
		<p class="text-xs text-muted-foreground">{hint}</p>
	{/if}
</div>
