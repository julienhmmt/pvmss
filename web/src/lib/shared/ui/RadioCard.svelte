<script lang="ts">
	import type { Snippet } from 'svelte';
	/**
	 * RadioCard - full-width `<label>` wrapper around a visually-hidden
	 * radio. Used by the create form for offering and size selection
	 * (DESIGN.md §8 "Radio cards"). Selecting is done by clicking anywhere
	 * on the card; the radio stays accessible to the keyboard.
	 *
	 * The caller owns `group` (the current selection) and passes `selected`
	 * (`group === value`) for the visual state. Selection is one-directional
	 * back to the parent via `onSelect` - Svelte 5 does not two-way-bind
	 * radios across components cleanly, so the parent's `group` setter is
	 * the single source of truth. `name` is a stable group identifier
	 * (e.g. "offering", "size") so all radios in a group share a form
	 * control name regardless of the current value.
	 */
	type Size = 'sm' | 'md';

	interface Props {
		/** Value this card represents. */
		value: string;
		/** Stable form-control name shared by every radio in the group. */
		name: string;
		/** True when `group === value` - drives the selected styling. */
		selected: boolean;
		/** Fired when the user picks this card; the parent updates `group`. */
		onSelect: (value: string) => void;
		size?: Size;
		disabled?: boolean;
		/** Card title row (e.g. the offering name). */
		header: Snippet;
		/** Card body (e.g. the spec line). */
		children: Snippet;
		/** Optional trailing affordance (e.g. a price). */
		trailing?: Snippet;
	}

	let {
		value,
		name,
		selected,
		onSelect,
		size = 'md',
		disabled = false,
		header,
		children,
		trailing
	}: Props = $props();

	const sizes: Record<Size, { pad: string; gap: string }> = {
		sm: { pad: 'p-3', gap: 'gap-1.5' },
		md: { pad: 'p-4', gap: 'gap-2' }
	};
</script>

<label
	class="relative flex cursor-pointer items-start {sizes[size].pad} {sizes[size].gap} rounded-lg border transition-colors
		{disabled
		? 'cursor-not-allowed border-border bg-muted/40 opacity-60'
		: selected
			? 'border-primary bg-sidebar-accent'
			: 'border-border bg-card hover:border-foreground/30'}
		focus-within:outline-none focus-within:ring-[3px] focus-within:ring-ring focus-within:ring-offset-3 focus-within:ring-offset-background"
>
	<input
		type="radio"
		class="sr-only"
		{name}
		{value}
		checked={selected}
		{disabled}
		onchange={() => {
			if (!disabled) onSelect(value);
		}}
	/>
	<span
		class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border transition-colors
			{selected ? 'border-primary' : 'border-muted-foreground/40'}"
		aria-hidden="true"
	>
		{#if selected}
			<span class="h-2 w-2 rounded-full bg-primary"></span>
		{/if}
	</span>
	<span class="flex min-w-0 flex-1 flex-col {sizes[size].gap}">
		<span class="flex items-center justify-between gap-3">
			<span class="font-medium text-foreground">{@render header()}</span>
			{#if trailing}<span class="shrink-0 text-sm text-muted-foreground">{@render trailing()}</span>{/if}
		</span>
		<span class="text-sm text-muted-foreground">{@render children()}</span>
	</span>
</label>
