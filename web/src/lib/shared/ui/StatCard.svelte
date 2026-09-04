<script lang="ts">
	/**
	 * StatCard — a single headline number with its label.
	 *
	 * The dashboard was hand-rolling `<div class="rounded-lg border … p-4">`
	 * with a muted `<p>` label above a `text-3xl` value, three times, with a
	 * fourth copy that had a popover bolted on. This is that block, with the
	 * label first as a small caps eyebrow (the number is what the eye should
	 * land on, so it gets the weight and the mono tabular figures), and an
	 * optional `hint` line for the unit or denominator.
	 *
	 * Pass `href` to make the whole tile a link — it then lifts on hover
	 * rather than only underlining its text.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** Small label above the number. */
		label: string;
		/** The headline value. Rendered in tabular mono figures. */
		value: string | number;
		/** Secondary line under the value (unit, denominator, freshness). */
		hint?: string;
		/** Optional leading icon, rendered in a tinted disc. */
		icon?: Snippet;
		/** Makes the tile a link. */
		href?: string;
		/** Extra content under the value (a meter, a breakdown). */
		children?: Snippet;
		class?: string;
		[key: string]: unknown;
	}

	let { label, value, hint, icon, href, children, class: klass = '', ...rest }: Props = $props();

	const shell =
		'block rounded-xl border border-border bg-card p-4 text-left shadow-card transition-[box-shadow,border-color] duration-150';
	const linkShell = 'hover:border-muted-foreground-subtle hover:shadow-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background';
</script>

{#snippet body()}
	<div class="flex items-start justify-between gap-3">
		<div class="min-w-0">
			<p class="text-[0.6875rem] font-semibold uppercase tracking-[0.06em] text-muted-foreground">
				{label}
			</p>
			<p class="mt-1.5 font-mono text-3xl font-semibold leading-none tracking-tight tabular-nums">
				{value}
			</p>
			{#if hint}
				<p class="mt-1.5 text-xs text-muted-foreground">{hint}</p>
			{/if}
		</div>
		{#if icon}
			<span
				class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"
				aria-hidden="true"
			>
				{@render icon()}
			</span>
		{/if}
	</div>
	{#if children}
		<div class="mt-3">{@render children()}</div>
	{/if}
{/snippet}

{#if href}
	<a {href} class="{shell} {linkShell} {klass}" {...rest}>{@render body()}</a>
{:else}
	<div class="{shell} {klass}" {...rest}>{@render body()}</div>
{/if}
