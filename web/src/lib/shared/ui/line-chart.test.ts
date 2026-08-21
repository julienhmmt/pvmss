import { describe, expect, it } from 'vitest';
import { buildLineChartPath } from './line-chart';

describe('buildLineChartPath', () => {
	it('returns an empty path for no values', () => {
		expect(buildLineChartPath([], 100, 40)).toBe('');
	});

	it('draws a flat horizontal line for a single value', () => {
		expect(buildLineChartPath([5], 100, 40)).toBe('M0,20 L100,20');
	});

	it('draws a flat horizontal line for a constant series (no divide-by-zero)', () => {
		const path = buildLineChartPath([3, 3, 3], 100, 40);
		expect(path).toBe('M0.00,20.00 L50.00,20.00 L100.00,20.00');
	});

	it('maps the minimum value to the bottom and the maximum to the top', () => {
		const path = buildLineChartPath([0, 10], 100, 40);
		expect(path).toBe('M0.00,40.00 L100.00,0.00');
	});

	it('spaces points evenly across the width', () => {
		const path = buildLineChartPath([0, 10, 0, 10], 90, 40);
		expect(path).toBe('M0.00,40.00 L30.00,0.00 L60.00,40.00 L90.00,0.00');
	});
});
