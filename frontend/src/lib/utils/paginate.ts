/**
 * Returns the slice of items for the requested page.
 *
 * @param items - Full array of items to paginate.
 * @param page - 1-indexed current page.
 * @param perPage - Number of items per page.
 * @returns The items belonging to the requested page.
 */
export function paginate<T>(items: readonly T[], page: number, perPage: number): T[] {
	if (perPage <= 0) return [];
	const start = Math.max(0, (page - 1) * perPage);
	return items.slice(start, start + perPage);
}
