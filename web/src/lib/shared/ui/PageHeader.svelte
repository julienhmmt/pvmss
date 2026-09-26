<script lang="ts">
	/**
	 * PageHeader - the canonical page title row for admin pages.
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
		/** Optional back link above the eyebrow (e.g. "My machines"). */
		back?: { href: string; label: string };
		/** Make the <h1> the screen-change focus target (`#page-heading`,
		 *  tabindex -1, DESIGN.md §9). Ignored when titleId is set. */
		focusTarget?: boolean;
	}

	let { title, eyebrow, description, titleId, actions, divider = true, back, focusTarget = false }: Props = $props();
</script>

<div class="mb-6 {divider ? 'border-b border-border pb-5' : ''}">
	<div class="flex flex-wrap items-end justify-between gap-x-4 gap-y-3">
		<div class="min-w-0">
			{#if back}
				<a
					href={back.href}
					class="pv-focus mb-3 inline-flex items-center gap-1 rounded text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
					data-testid="page-back-link"
				>
					<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4" aria-hidden="true">
						<path d="M12 5l-5 5 5 5" />
					</svg>
					{back.label}
				</a>
			{/if}
			{#if eyebrow}
				<p class="mb-1 text-[0.6875rem] font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle">
					{eyebrow}
				</p>
			{/if}
			{#if focusTarget && !titleId}
				<h1 id="page-heading" tabindex="-1" class="text-2xl font-semibold tracking-tight text-balance focus:outline-none">{title}</h1>
			{:else}
				<h1 id={titleId} class="text-2xl font-semibold tracking-tight text-balance">{title}</h1>
			{/if}
			{#if description}
				<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{description}</p>
			{/if}
		</div>
		{#if actions}
			<div class="flex max-w-full min-w-0 flex-wrap items-center gap-2">{@render actions()}</div>
		{/if}
	</div>
</div>
