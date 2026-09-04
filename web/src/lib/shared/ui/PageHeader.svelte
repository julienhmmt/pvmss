<script lang="ts">
	/**
	 * PageHeader — the canonical page title row for admin pages.
	 *
	 * Before this, every admin page hand-rolled the same
	 * `<h1 class="text-2xl font-semibold tracking-tight">` block, sometimes
	 * with a right-aligned action (a ClusterSelector) and sometimes a
	 * description line. Extracting it stops 15 copies from drifting apart.
	 *
	 * The header carries the page's vertical rhythm: an optional eyebrow
	 * (section name / breadcrumb tail), the title, the description, and a
	 * hairline rule that separates the header from the content below. The
	 * rule is what makes a page read as "header, then content" rather than
	 * as a stack of equally-weighted blocks.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** The page title, rendered as the single <h1>. */
		title: string;
		/** Small uppercase label above the title (section, cluster, owner). */
		eyebrow?: string;
		/** Optional supporting line under the title. */
		description?: string;
		/** Optional id for the <h1> so a section can aria-labelledby it. */
		titleId?: string;
		/** Optional right-aligned actions (e.g. a cluster selector, a button). */
		actions?: Snippet;
		/** Draw the separating rule. Off for pages that open on a full-bleed card. */
		divider?: boolean;
	}

	let { title, eyebrow, description, titleId, actions, divider = true }: Props = $props();
</script>

<div class="mb-6 {divider ? 'border-b border-border pb-5' : ''}">
	<div class="flex flex-wrap items-end justify-between gap-x-4 gap-y-3">
		<div class="min-w-0">
			{#if eyebrow}
				<p class="mb-1 text-[0.6875rem] font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle">
					{eyebrow}
				</p>
			{/if}
			<h1 id={titleId} class="text-2xl font-semibold tracking-tight text-balance">{title}</h1>
			{#if description}
				<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{description}</p>
			{/if}
		</div>
		{#if actions}
			<div class="flex shrink-0 flex-wrap items-center gap-2">{@render actions()}</div>
		{/if}
	</div>
</div>
