<script lang="ts">
	/**
	 * Card — Layer B repeating primitive (mockup `.crd`): white surface, 1 px
	 * `--border`, `--radius`, soft two-layer shadow. No extra role; headings
	 * stay headings. Padding is opt-in via the `pad` prop so list/table cards
	 * can sit flush while content cards get breathing room.
	 *
	 * `title`/`description`/`actions` render a header band separated by a
	 * hairline — the pattern ~20 pages were hand-rolling as a `<div class="flex
	 * items-center justify-between border-b …">`. `interactive` is for cards
	 * that are themselves a link or button target: they lift on hover instead
	 * of only changing their background.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** Inner padding preset for the body. `none` lets tables/lists sit flush. */
		pad?: 'none' | 'sm' | 'md' | 'lg';
		/** Render as a section/article/aside/etc. Default `section`. */
		as?: 'section' | 'article' | 'aside' | 'div';
		/** Header title. Renders the header band when set. */
		title?: string;
		/** Heading level for `title`. Default 2 — pick the one the page needs. */
		titleAs?: 'h2' | 'h3' | 'h4';
		/** Optional id on the heading so a section can aria-labelledby it. */
		titleId?: string;
		/** Supporting line under the title. */
		description?: string;
		/** Right-aligned header content (buttons, selectors). */
		actions?: Snippet;
		/** Full custom header, replacing title/description/actions. */
		header?: Snippet;
		/** Bordered footer band (dialog-style action rows). */
		footer?: Snippet;
		/** Lift on hover — for cards that are a link or button target. */
		interactive?: boolean;
		class?: string;
		children: Snippet;
	}

	let {
		pad = 'md',
		as = 'section',
		title,
		titleAs = 'h2',
		titleId,
		description,
		actions,
		header,
		footer,
		interactive = false,
		class: extra = '',
		children
	}: Props = $props();

	const padding: Record<NonNullable<Props['pad']>, string> = {
		none: '',
		sm: 'p-4',
		md: 'p-5',
		lg: 'p-6'
	};

	const hasHeader = $derived(header !== undefined || title !== undefined);
	const banded = $derived(hasHeader || footer !== undefined);
</script>

<svelte:element
	this={as}
	class="rounded-xl border border-border bg-card text-card-foreground shadow-card {banded
		? 'overflow-hidden'
		: padding[pad]} {interactive
		? 'transition-[box-shadow,border-color] duration-150 hover:border-muted-foreground-subtle hover:shadow-raised'
		: ''} {extra}"
>
	{#if header}
		<div class="border-b border-border px-5 py-4">{@render header()}</div>
	{:else if title}
		<div class="flex flex-wrap items-start justify-between gap-3 border-b border-border px-5 py-4">
			<div class="min-w-0">
				<svelte:element this={titleAs} id={titleId} class="text-sm font-semibold tracking-tight text-foreground">
					{title}
				</svelte:element>
				{#if description}
					<p class="mt-1 text-xs text-muted-foreground">{description}</p>
				{/if}
			</div>
			{#if actions}
				<div class="flex shrink-0 items-center gap-2">{@render actions()}</div>
			{/if}
		</div>
	{/if}

	{#if banded}
		<div class={padding[pad]}>{@render children()}</div>
	{:else}
		{@render children()}
	{/if}

	{#if footer}
		<div class="flex flex-wrap items-center justify-end gap-2 border-t border-border bg-muted/40 px-5 py-3">
			{@render footer()}
		</div>
	{/if}
</svelte:element>
