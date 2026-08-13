<script lang="ts">
	/**
	 * PageHeader — the canonical page title row for admin pages.
	 *
	 * Before this, every admin page hand-rolled the same
	 * `<h1 class="text-2xl font-semibold tracking-tight">` block, sometimes
	 * with a right-aligned action (a ClusterSelector) and sometimes a
	 * description line. Extracting it stops 15 copies from drifting apart.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** The page title, rendered as the single <h1>. */
		title: string;
		/** Optional supporting line under the title. */
		description?: string;
		/** Optional id for the <h1> so a section can aria-labelledby it. */
		titleId?: string;
		/** Optional right-aligned actions (e.g. a cluster selector, a button). */
		actions?: Snippet;
	}

	let { title, description, titleId, actions }: Props = $props();
</script>

<div class="mb-6">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h1 id={titleId} class="text-2xl font-semibold tracking-tight">{title}</h1>
		{#if actions}{@render actions()}{/if}
	</div>
	{#if description}
		<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{description}</p>
	{/if}
</div>
