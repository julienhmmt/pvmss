/**
 * Two-letter initials for an account avatar: first letters of the first two
 * name parts ("Jane Doe" -> "JD"), or the first two letters of a single part
 * ("alice" -> "AL"). "?" for an empty name.
 */
export function accountInitials(name: string): string {
	const parts = name.trim().split(/[\s._-]+/).filter(Boolean);
	const first = parts[0] ?? '';
	if (first === '') return '?';
	const second = parts.length > 1 ? (parts[1]?.[0] ?? '') : (first[1] ?? '');
	return `${first[0] ?? ''}${second}`.toUpperCase();
}
