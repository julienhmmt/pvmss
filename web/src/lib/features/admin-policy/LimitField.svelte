<script lang="ts">
	/**
	 * LimitField - one policy limit row: a FieldRow (label + hint + error on
	 * the left) wrapping the number TextField on the right. Keeps every
	 * numeric limit on the page at one width with an optional unit suffix.
	 */
	import FieldRow from '#lib/shared/ui/FieldRow.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';

	interface Props {
		/** Visible label text. */
		label: string;
		/** Bound value - a number, or a string getter/setter pair for display conversions. */
		value: string | number;
		min?: number;
		max?: number;
		step?: number;
		/** Hint text shown under the label. */
		hint?: string;
		/** Error message; marks the control invalid. */
		error?: string | null;
		/** Unit rendered inside the input (e.g. "GiB"). */
		unit?: string;
		disabled?: boolean;
		placeholder?: string;
	}

	let {
		label,
		value = $bindable(),
		min,
		max,
		step,
		hint,
		error = null,
		unit,
		disabled = false,
		placeholder
	}: Props = $props();
</script>

<FieldRow {label} {hint} {error}>
	{#snippet children({ id, describedBy, invalid })}
		<TextField
			{id}
			{describedBy}
			{invalid}
			type="number"
			{min}
			{max}
			{step}
			{disabled}
			{placeholder}
			bind:value
			class="w-full sm:w-36"
		>
			{#snippet trailing()}
				{#if unit}<span class="text-xs text-muted-foreground">{unit}</span>{/if}
			{/snippet}
		</TextField>
	{/snippet}
</FieldRow>
