<script lang="ts">
	/**
	 * LineChart — a small, dependency-free SVG sparkline. Thin wrapper around
	 * buildLineChartPath; scales via viewBox so it stretches to fill its
	 * container (h-full w-full via the class prop).
	 */
	import { buildLineChartPath } from './line-chart';

	interface Props {
		values: number[];
		label: string;
		width?: number;
		height?: number;
		class?: string;
	}

	let { values, label, width = 200, height = 48, class: className = 'h-12 w-full' }: Props = $props();

	const path = $derived(buildLineChartPath(values, width, height));
</script>

<svg
	viewBox="0 0 {width} {height}"
	preserveAspectRatio="none"
	class={className}
	role="img"
	aria-label={label}
	data-testid="line-chart"
>
	{#if path}
		<path d={path} fill="none" stroke="currentColor" stroke-width="1.5" class="text-primary" vector-effect="non-scaling-stroke" />
	{/if}
</svg>
