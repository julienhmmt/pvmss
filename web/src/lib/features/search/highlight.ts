/**
 * Match highlighting for the search result table. Mirrors the server's
 * searchVMs rules exactly: a substring match against name and tags, and an
 * exact numeric match against the VMID - so "2222" never highlights part of
 * VM 2222001, because the server would not have returned that row for it.
 */

export interface HighlightSegment {
	text: string;
	match: boolean;
}

/** Splits `text` at every case-insensitive occurrence of the trimmed query. */
export function highlightText(text: string, query: string): HighlightSegment[] {
	const needle = query.trim().toLowerCase();
	if (needle === '' || text === '') return [{ text, match: false }];
	const lowered = text.toLowerCase();
	const segments: HighlightSegment[] = [];
	let cursor = 0;
	for (;;) {
		const found = lowered.indexOf(needle, cursor);
		if (found === -1) break;
		if (found > cursor) segments.push({ text: text.slice(cursor, found), match: false });
		segments.push({ text: text.slice(found, found + needle.length), match: true });
		cursor = found + needle.length;
	}
	if (cursor < text.length) segments.push({ text: text.slice(cursor), match: false });
	return segments;
}

/** VMIDs match only on an exact numeric query, not on a substring. */
export function vmidMatches(vmid: number, query: string): boolean {
	const trimmed = query.trim();
	return /^\d+$/.test(trimmed) && Number(trimmed) === vmid;
}
