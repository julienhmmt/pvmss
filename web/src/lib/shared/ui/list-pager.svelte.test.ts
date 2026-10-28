import { flushSync } from 'svelte';
import { beforeEach, describe, expect, it } from 'vitest';
import { createPagination } from './list-pager.svelte';

const all = Array.from({ length: 56 }, (_, i) => i + 1);

describe('createPagination', () => {
	beforeEach(() => localStorage.clear());

	it('slices rows and follows page changes', () => {
		const cleanup = $effect.root(() => {
			const pager = createPagination('t-slice', () => all);
			flushSync();
			expect(pager.total).toBe(56);
			expect(pager.rows).toHaveLength(25);
			pager.page = 3;
			flushSync();
			expect(pager.rows).toEqual([51, 52, 53, 54, 55, 56]);
		});
		cleanup();
	});

	it('resets to page 1 and persists when the size changes', () => {
		const cleanup = $effect.root(() => {
			const pager = createPagination('t-size', () => all);
			flushSync();
			pager.page = 3;
			flushSync();
			pager.size = 10;
			flushSync();
			expect(pager.page).toBe(1);
			expect(pager.rows).toHaveLength(10);
			expect(localStorage.getItem('pvmss.pageSize.t-size')).toBe('10');
		});
		cleanup();
	});

	it('resets to page 1 when the source total changes', () => {
		const cleanup = $effect.root(() => {
			let data = $state(all);
			const pager = createPagination('t-total', () => data);
			flushSync();
			pager.page = 2;
			flushSync();
			data = all.slice(0, 30);
			flushSync();
			expect(pager.page).toBe(1);
			expect(pager.total).toBe(30);
			expect(pager.rows).toHaveLength(25);
		});
		cleanup();
	});

	it('remembers the stored size', () => {
		localStorage.setItem('pvmss.pageSize.t-stored', '50');
		const cleanup = $effect.root(() => {
			const pager = createPagination('t-stored', () => all);
			expect(pager.size).toBe(50);
		});
		cleanup();
	});
});
