<script lang="ts">
	/**
	 * OptionCard — Layer B large selectable profile card (mockup `.opt`). A
	 * visual restyle of a native radio, not a custom dual-state widget: the
	 * input stays `<input type="radio">` inside a `<label>`, so keyboard /
	 * screen-reader behaviour comes for free. The parent groups radios in a
	 * `<fieldset role="radiogroup">` and binds `group`.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** Value this radio submits. */
		value: string;
		/** Current group value (bind:group on the input). */
		group: string;
		/** Accessible label for the radio — also the visible title. */
		label: string;
		/** Optional description shown under the title. */
		description?: string;
		/** Optional leading snippet (icon / glyph). */
		media?: Snippet;
	}

	let { value, group, label, description, media }: Props = $props();

	const selected = $derived(group === value);
</script>

<label
	class="flex cursor-pointer items-start gap-3 rounded-xl border p-4 transition-colors focus-within:ring-2 focus-within:ring-ring {selected
		? 'border-primary bg-sidebar-accent'
		: 'border-border bg-card hover:border-primary/50'}"
	data-on={selected ? '1' : undefined}
>
	<input
		type="radio"
		{value}
		bind:group
		class="mt-0.5 h-4 w-4 accent-primary focus-visible:outline-none"
		aria-label={label}
	/>
	<span class="flex flex-1 flex-col gap-0.5">
		{#if media}<span class="text-muted-foreground">{@render media()}</span>{/if}
		<span class="text-sm font-semibold text-foreground">{label}</span>
		{#if description}<span class="text-sm text-muted-foreground">{description}</span>{/if}
	</span>
</label>
