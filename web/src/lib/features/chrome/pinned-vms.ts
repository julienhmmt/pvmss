/**
 * Pinned machines for the sidebar drawer. A per-browser convenience, so it
 * lives in localStorage (keyed per user) rather than on the server; storage
 * that throws or holds junk reads as "nothing pinned".
 */

const KEY_PREFIX = 'pvmss-pinned-vms-v1:';

export function pinKey(vm: { cluster: string; vmid: number }): string {
	return `${vm.cluster}/${vm.vmid}`;
}

export function readPins(user: string): ReadonlySet<string> {
	try {
		const parsed: unknown = JSON.parse(localStorage.getItem(KEY_PREFIX + user) ?? '[]');
		return new Set(Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : []);
	} catch {
		return new Set();
	}
}

/** Returns the new set; the caller's set is left untouched. */
export function togglePin(user: string, pins: ReadonlySet<string>, key: string): ReadonlySet<string> {
	const next = new Set(pins);
	if (next.has(key)) next.delete(key);
	else next.add(key);
	try {
		localStorage.setItem(KEY_PREFIX + user, JSON.stringify([...next]));
	} catch {
		// Private mode or blocked storage: the pin still holds for this page view.
	}
	return next;
}

/** Pinned machines first (in their existing order), then up to `limit` others. */
export function orderPinnedFirst<T extends { cluster: string; vmid: number }>(
	vms: readonly T[],
	pins: ReadonlySet<string>,
	limit: number
): T[] {
	const pinned = vms.filter((vm) => pins.has(pinKey(vm)));
	const rest = vms.filter((vm) => !pins.has(pinKey(vm))).slice(0, limit);
	return [...pinned, ...rest];
}
