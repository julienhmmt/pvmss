<script lang="ts">
	import { nextFieldId } from '$lib/shared/ui/field-id';

	interface Props {
		text: string;
		position?: 'top' | 'bottom';
		children?: import('svelte').Snippet;
	}

	let { text, position = 'top', children }: Props = $props();

	const tooltipId = nextFieldId('tooltip');
</script>

<span class="group relative inline-flex" aria-describedby={tooltipId}>
	{#if children}{@render children()}{/if}
	<span
		id={tooltipId}
		role="tooltip"
		class="pointer-events-none absolute left-1/2 z-50 -translate-x-1/2 whitespace-nowrap rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground opacity-0 shadow-card transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100 {position === 'top' ? 'bottom-full mb-1' : 'top-full mt-1'}"
	>
		{text}
	</span>
</span>
