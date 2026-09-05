<script lang="ts">
	/**
	 * TableCard — shared shell for admin data tables. Renders a rounded
	 * card with shadow, an optional bordered toolbar header, and a
	 * horizontally scrollable table body. Use the `toolbar` snippet for
	 * filters/search and the default children for the `<table>` element.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** Rendered in a bordered header bar above the table. Omit to hide. */
		toolbar?: Snippet;
		/** The table content (a `<table>` element). */
		children: Snippet;
		/** Extra classes on the outer card. */
		class?: string;
	}

	let { toolbar, children, class: extra = '' }: Props = $props();
</script>

<section class="overflow-hidden rounded-lg border border-border bg-card shadow-card {extra}">
	{#if toolbar}
		<div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
			{@render toolbar()}
		</div>
	{/if}
	<div class="overflow-x-auto">
		{@render children()}
	</div>
</section>
