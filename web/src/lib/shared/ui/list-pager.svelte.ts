import { loadPageSize, paginate, savePageSize } from './paginate';

/** Reactive pagination state for one list. Bind `pager` to <Pagination>. */
export interface ListPager<T> {
	/** Rows of the current page. */
	readonly rows: T[];
	/** Total items in the source list. */
	readonly total: number;
	/** 1-based current page. */
	page: number;
	/** Page size; 0 = all. Writes are persisted per list name. */
	size: number;
}

/**
 * Call during component init. `name` keys the remembered page size;
 * `source` returns the (already filtered) full list. The page resets to 1
 * whenever the total or the size changes.
 */
export function createPagination<T>(name: string, source: () => readonly T[]): ListPager<T> {
	let page = $state(1);
	let size = $state(loadPageSize(name));
	const total = $derived(source().length);
	const rows = $derived(paginate(source(), page, size));

	$effect(() => {
		void total;
		void size;
		page = 1;
	});

	return {
		get rows() {
			return rows;
		},
		get total() {
			return total;
		},
		get page() {
			return page;
		},
		set page(value: number) {
			page = value;
		},
		get size() {
			return size;
		},
		set size(value: number) {
			size = value;
			savePageSize(name, value);
		}
	};
}
