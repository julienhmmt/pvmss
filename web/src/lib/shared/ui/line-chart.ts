/**
 * Builds an SVG `<path>` `d` attribute for a simple line chart: values are
 * normalized to the [0, height] range (min → bottom, max → top) and spaced
 * evenly across [0, width]. Pure and framework-free so it stays testable
 * without mounting a component (LineChart.svelte just renders the result).
 */
export function buildLineChartPath(values: number[], width: number, height: number): string {
	if (values.length === 0) return '';

	if (values.length === 1) {
		const y = height / 2;
		return `M0,${y} L${width},${y}`;
	}

	const min = Math.min(...values);
	const max = Math.max(...values);
	const range = max - min;
	const stepX = width / (values.length - 1);

	return values
		.map((value, index) => {
			const x = index * stepX;
			// A constant series (range 0) draws flat at mid-height, not pinned
			// to the computed minimum — that would misleadingly read as "low".
			const y = range === 0 ? height / 2 : height - ((value - min) / range) * height;
			return `${index === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`;
		})
		.join(' ');
}
