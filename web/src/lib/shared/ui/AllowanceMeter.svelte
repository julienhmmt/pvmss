<script lang="ts" module>
	/**
	 * AllowanceMeter - a segmented `role="meter"` bar. Renders `limit`
	 * segments; `used` of them are filled in accent. Used in the list
	 * footer and the create summary (DESIGN.md §8 "Allowance meter").
	 *
	 * Sibling to `Meter.svelte` (which is a continuous quota bar). The
	 * two share the `role="meter"` + aria contract but differ in shape:
	 * Meter is a continuous fill for a 0–100 percentage; AllowanceMeter
	 * is a fixed count of slots (e.g. "3 of 5 machines used"). Reusing
	 * Meter's `quotaMeterView` here would be wrong - that helper models a
	 * percentage of a bound, not a slot count.
	 */
	export interface AllowanceMeterProps {
		used: number;
		/** Total number of segments. Must be > 0. */
		limit: number;
		/** Accessible label, e.g. "3 of 5 machines used". */
		label: string;
	}
</script>

<script lang="ts">
	let { used, limit, label }: AllowanceMeterProps = $props();

	const clampedUsed = $derived(Math.max(0, Math.min(used, limit)));
	const segments = $derived(Array.from({ length: limit }, (_, i) => i < clampedUsed));
</script>

<div
	class="flex items-center gap-1"
	role="meter"
	aria-valuenow={clampedUsed}
	aria-valuemin={0}
	aria-valuemax={limit}
	aria-label={label}
>
	{#each segments as filled, i (i)}
		<span
			class="h-1.5 flex-1 rounded-full transition-colors motion-reduce:transition-none
				{filled ? 'bg-primary' : 'bg-muted'}"
		></span>
	{/each}
</div>
