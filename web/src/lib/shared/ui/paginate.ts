export const PAGE_SIZES = [10, 25, 50, 100, 0] as const; // 0 = All
export const DEFAULT_PAGE_SIZE = 25;

/** Slice one page out of items. Page is 1-based and clamped; size 0 = all. */
export function paginate<T>(items: readonly T[], page: number, size: number): T[] {
	if (size <= 0) return items.slice();
	const last = Math.max(1, Math.ceil(items.length / size));
	const p = Math.min(Math.max(1, Math.floor(page) || 1), last);
	return items.slice((p - 1) * size, p * size);
}

const key = (name: string): string => `pvmss.pageSize.${name}`;

export function loadPageSize(name: string): number {
	try {
		const raw = localStorage.getItem(key(name));
		const v = Number(raw);
		return raw !== null && (PAGE_SIZES as readonly number[]).includes(v) ? v : DEFAULT_PAGE_SIZE;
	} catch {
		return DEFAULT_PAGE_SIZE;
	}
}

export function savePageSize(name: string, size: number): void {
	try {
		localStorage.setItem(key(name), String(size));
	} catch {
		// storage unavailable: the size just won't persist
	}
}
