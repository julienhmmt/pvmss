<script lang="ts">
	/**
	 * NodeUsageBar — a small meter for node capacity (CPU or memory).
	 * Value is 0–1. Colours shift at the low/high thresholds using the design
	 * system semantic tokens (success / warning / destructive).
	 */
	interface Props {
		/** Usage ratio between 0 and 1. */
		value: number;
		/** Visible and accessible label rendered above the bar. */
		label: string;
		/** Threshold where the bar turns from success to warning (default 0.6). */
		low?: number;
		/** Threshold where the bar turns from warning to destructive (default 0.85). */
		high?: number;
	}

	let { value, label, low = 0.6, high = 0.85 }: Props = $props();

	const percent = $derived(Math.min(100, Math.round(value * 100)));
	const barColor = $derived(
		percent >= high * 100 ? 'bg-destructive' : percent >= low * 100 ? 'bg-warning' : 'bg-success'
	);
</script>

<div class="flex flex-col gap-1">
	<span class="text-xs text-muted-foreground">{label}</span>
	<div
		class="h-1.5 w-24 overflow-hidden rounded-full bg-muted"
		role="meter"
		aria-valuemin={0}
		aria-valuemax={100}
		aria-valuenow={percent}
		aria-label={label}
	>
		<div
			class="h-full rounded-full {barColor} transition-[width] motion-reduce:transition-none"
			style="width: {percent}%"
		></div>
	</div>
</div>
