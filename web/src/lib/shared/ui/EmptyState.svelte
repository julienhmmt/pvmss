<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * EmptyState — a teaching empty-state for zero-row result sets, distinct
	 * from the loading and error states. Optional icon snippet, a title, a
	 * description, and an optional actions snippet (e.g. the "New …" button)
	 * so the empty state itself is the call to action.
	 */
	interface Props {
		title: string;
		description?: string;
		icon?: Snippet;
		actions?: Snippet;
		class?: string;
		dataTestid?: string;
	}

	let { title, description, icon, actions, class: className = '', dataTestid }: Props = $props();
</script>

<div class="flex flex-col items-center justify-center gap-3 px-4 py-12 text-center {className}" data-testid={dataTestid}>
	{#if icon}
		<div class="text-muted-foreground-subtle">{@render icon()}</div>
	{/if}
	<div>
		<p class="font-semibold">{title}</p>
		{#if description}
			<p class="mt-1 text-sm text-muted-foreground">{description}</p>
		{/if}
	</div>
	{#if actions}
		<div class="mt-2">{@render actions()}</div>
	{/if}
</div>
