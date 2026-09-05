<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * EmptyState — a teaching empty-state for zero-row result sets, distinct
	 * from the loading and error states. Optional icon snippet, a title, a
	 * description, and an optional actions snippet (e.g. the "New …" button)
	 * so the empty state itself is the call to action.
	 *
	 * The icon sits in a tinted disc so an empty table does not read as a
	 * loading failure: the disc is a deliberate, finished-looking shape,
	 * where a bare grey glyph on white reads as something that failed to
	 * render. `tone="error"` swaps the disc to the destructive soft triple
	 * for unreachable-cluster style states.
	 */
	type Tone = 'neutral' | 'error';

	interface Props {
		title: string;
		description?: string;
		icon?: Snippet;
		actions?: Snippet;
		tone?: Tone;
		class?: string;
		dataTestid?: string;
	}

	let {
		title,
		description,
		icon,
		actions,
		tone = 'neutral',
		class: className = '',
		dataTestid
	}: Props = $props();

	const discTone: Record<Tone, string> = {
		neutral: 'border-border bg-muted text-muted-foreground',
		error: 'border-destructive-soft-border bg-destructive-soft text-destructive-soft-foreground'
	};
</script>

<div
	class="flex flex-col items-center justify-center gap-4 px-6 py-14 text-center {className}"
	data-testid={dataTestid}
>
	{#if icon}
		<div
			class="flex h-12 w-12 items-center justify-center rounded-full border {discTone[tone]}"
			aria-hidden="true"
		>
			{@render icon()}
		</div>
	{/if}
	<div class="max-w-md">
		<p class="text-base font-semibold text-foreground">{title}</p>
		{#if description}
			<p class="mt-1.5 text-sm text-muted-foreground">{description}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex flex-wrap items-center justify-center gap-2">{@render actions()}</div>
	{/if}
</div>
