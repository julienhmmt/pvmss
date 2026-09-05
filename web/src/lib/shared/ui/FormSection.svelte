<script lang="ts">
	/**
	 * FormSection — a titled group of fields inside a longer form.
	 *
	 * The create-VM wizard and the admin forms were flat `grid gap-4` stacks:
	 * ten controls of identical weight with nothing saying which ones belong
	 * together. A form that long needs chapters. This renders a real
	 * `<fieldset>`/`<legend>` (so assistive tech gets the grouping for free)
	 * with an optional description and a numbered marker for wizard steps.
	 *
	 * `variant="plain"` is the default — a legend, a hairline, and the
	 * fields. `variant="panel"` puts the group on the muted ground for
	 * secondary or advanced settings that should read as an aside.
	 */
	import type { Snippet } from 'svelte';

	type Variant = 'plain' | 'panel';

	interface Props {
		/** Group name, rendered as the <legend>. */
		legend: string;
		/** Supporting line under the legend. */
		description?: string;
		/** Step number shown in a disc before the legend (wizards). */
		step?: number;
		variant?: Variant;
		/** Right-aligned control on the legend row (a Switch, a reset link). */
		actions?: Snippet;
		class?: string;
		children: Snippet;
	}

	let {
		legend,
		description,
		step,
		variant = 'plain',
		actions,
		class: klass = '',
		children
	}: Props = $props();

	// The panel chrome lives on a wrapper, never on the <fieldset> itself: a
	// <legend> is painted over its fieldset's top border and cuts a notch in
	// it, which reads as a rendering bug on a rounded, filled panel.
	const shells: Record<Variant, string> = {
		plain: '',
		panel: 'rounded-xl border border-border bg-muted/40 p-4'
	};
</script>

<div class="min-w-0 {shells[variant]} {klass}">
<fieldset class="min-w-0">
	<!-- The <legend> stays a direct child of <fieldset> so the grouping is
	     real for assistive tech; `float-none w-full` undoes the UA's inline
	     shrink-wrap so the actions can sit on the same row. -->
	<legend class="float-none mb-0 w-full p-0">
		<span class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
			<span class="flex items-center gap-2 text-sm font-semibold text-foreground">
				{#if step !== undefined}
					<span
						class="flex h-5 w-5 items-center justify-center rounded-full bg-primary font-mono text-[0.6875rem] font-semibold text-primary-foreground"
						aria-hidden="true"
					>
						{step}
					</span>
				{/if}
				{legend}
			</span>
			{#if actions}
				<span class="flex items-center gap-2">{@render actions()}</span>
			{/if}
		</span>
	</legend>
	{#if description}
		<p class="mt-1 text-xs leading-relaxed text-muted-foreground">{description}</p>
	{/if}
	<div class="mt-4 grid gap-4">
		{@render children()}
	</div>
</fieldset>
</div>
