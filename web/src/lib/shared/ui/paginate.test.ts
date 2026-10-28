import { beforeEach, describe, expect, it } from 'vitest';
import { loadPageSize, paginate, savePageSize } from './paginate';

const items = Array.from({ length: 56 }, (_, i) => i + 1);

describe('paginate', () => {
	it('slices pages', () => {
		expect(paginate(items, 1, 25)).toHaveLength(25);
		expect(paginate(items, 3, 25)).toEqual([51, 52, 53, 54, 55, 56]);
	});
	it('clamps out-of-range pages', () => {
		expect(paginate(items, 99, 25)[0]).toBe(51);
		expect(paginate(items, 0, 25)[0]).toBe(1);
	});
	it('size 0 returns everything', () => {
		expect(paginate(items, 2, 0)).toHaveLength(56);
	});
	it('handles empty lists', () => {
		expect(paginate([], 1, 25)).toEqual([]);
	});
});

describe('page size storage', () => {
	beforeEach(() => localStorage.clear());
	it('defaults to 25 and round-trips', () => {
		expect(loadPageSize('x')).toBe(25);
		savePageSize('x', 50);
		expect(loadPageSize('x')).toBe(50);
		savePageSize('x', 0);
		expect(loadPageSize('x')).toBe(0);
	});
	it('ignores invalid stored values', () => {
		localStorage.setItem('pvmss.pageSize.y', '7');
		expect(loadPageSize('y')).toBe(25);
	});
});
