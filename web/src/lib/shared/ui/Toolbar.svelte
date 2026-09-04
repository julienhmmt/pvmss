<script lang="ts">
	/**
	 * Toolbar — the filter row that sits above every list in the app.
	 *
	 * Each list page was building its own: a `<div class="flex flex-wrap
	 * items-center gap-3 border-b border-border p-4">` holding a raw
	 * `<input type="search">` and two raw `<select>`s, each with slightly
	 * different padding and radius. This gives all of them one shape: search
	 * on the left at a capped width, filters next to it, a `meta` slot pushed
	 * to the right for counts and quotas, and `actions` at the far end.
	 *
	 * It renders no controls itself — pass TextField/Select/Button through
	 * the snippets so the controls stay the shared primitives.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** Leading slot — normally the search field. Capped at ~22rem. */
		search?: Snippet;
		/** Filter controls (selects, toggles). */
		filters?: Snippet;
		/** Right-aligned informational text (result counts, quota). */
		meta?: Snippet;
		/** Right-aligned buttons. */
		actions?: Snippet;
		/** Drop the bottom hairline — for a toolbar that is not on a card. */
		divider?: boolean;
		class?: string;
	}

	let { search, filters, meta, actions, divider = true, class: klass = '' }: Props = $props();
</script>

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 {divider
		? 'border-b border-border'
		: ''} {klass}"
>
	{#if search}
		<div class="w-full min-w-0 max-w-sm sm:w-auto sm:flex-1">{@render search()}</div>
	{/if}
	{#if filters}
		<div class="flex flex-wrap items-center gap-2">{@render filters()}</div>
	{/if}
	{#if meta || actions}
		<div class="ml-auto flex flex-wrap items-center gap-3">
			{#if meta}
				<div class="text-xs text-muted-foreground">{@render meta()}</div>
			{/if}
			{#if actions}
				<div class="flex items-center gap-2">{@render actions()}</div>
			{/if}
		</div>
	{/if}
</div>
