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
	import ButtonLink from './ButtonLink.svelte';
	import CinematicBackdrop from './CinematicBackdrop.svelte';

	interface Props {
		/** The page title, rendered as the single <h1>. */
		title: string;
		/** Small uppercase label above the title (section, cluster, owner). */
		eyebrow?: string;
		/** Optional supporting line under the title. */
		description?: string;
		/** Optional id for the <h1> so a section can aria-labelledby it. */
		titleId?: string;
		/** Optional right-aligned actions (e.g. a cluster selector, a button).
		 *  Sits on the back-button row when `back` is set, otherwise on the
		 *  title row. */
		actions?: Snippet;
		/** Draw the separating rule. Off for pages that open on a full-bleed card. */
		divider?: boolean;
		/** Optional back button above the eyebrow (e.g. "My machines"). */
		back?: { href: string; label: string };
		/** Inline element next to the title (e.g. a status Pill). */
		titleMeta?: Snippet;
		/** Make the <h1> the screen-change focus target (`#page-heading` or the
		 *  custom `titleId`, tabindex -1, DESIGN.md §9). On by default so every
		 *  page using PageHeader announces itself on client-side navigation. */
		focusTarget?: boolean;
		/** Login-style glow and dot texture behind the header. Landing pages only. */
		cinematic?: boolean;
	}

	let { title, eyebrow, description, titleId, actions, divider = true, back, titleMeta, focusTarget = true, cinematic = false }: Props = $props();
</script>

<div
	class="mb-6 {cinematic
		? 'relative isolate overflow-hidden rounded-xl border border-border bg-card px-5 py-4 min-[700px]:px-7 min-[700px]:py-5'
		: divider
			? 'border-b border-border pb-5'
			: ''}"
>
	{#if cinematic}<CinematicBackdrop />{/if}
	{#if back}
		<div class="relative mb-4 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
			<ButtonLink href={back.href} variant="secondary" size="sm" data-testid="page-back-link">
				{#snippet icon()}
					<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5" aria-hidden="true">
						<path d="M12 5l-5 5 5 5" />
					</svg>
				{/snippet}
				{back.label}
			</ButtonLink>
			{#if actions}
				<div class="flex min-w-0 flex-wrap items-center gap-2">{@render actions()}</div>
			{/if}
		</div>
	{/if}
	<div class="relative flex flex-wrap items-end justify-between gap-x-4 gap-y-3">
		<div class="min-w-0">
			{#if eyebrow}
				<p class="mb-1 text-2xs font-semibold uppercase {cinematic ? 'tracking-[0.16em] text-primary' : 'tracking-[0.08em] text-muted-foreground-subtle'}">
					{eyebrow}
				</p>
			{/if}
			{#if focusTarget}
				<h1 id={titleId ?? 'page-heading'} tabindex="-1" class="flex flex-wrap items-center gap-x-3 gap-y-1.5 {cinematic ? 'text-3xl tracking-[-0.035em]' : 'text-2xl tracking-tight'} font-semibold text-balance focus:outline-none">
					{title}
					{#if titleMeta}{@render titleMeta()}{/if}
				</h1>
			{:else}
				<h1 id={titleId} class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-2xl font-semibold tracking-tight text-balance">
					{title}
					{#if titleMeta}{@render titleMeta()}{/if}
				</h1>
			{/if}
			{#if description}
				<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{description}</p>
			{/if}
		</div>
		{#if actions && !back}
			<div class="flex max-w-full min-w-0 flex-wrap items-center gap-2">{@render actions()}</div>
		{/if}
	</div>
</div>
