import type { VmAuditEntry } from './detail.svelte';

const FORMULA_PREFIXES = ['=', '+', '-', '@'] as const;
const CSV_HEADER = 'timestamp,actor,action,label';

/** A cell that could be read as a formula by a spreadsheet gets a leading
 *  quote so the value stays text. */
function guardFormula(value: string): string {
	const isFormula = FORMULA_PREFIXES.some((prefix) => value.startsWith(prefix));
	return isFormula ? `'${value}` : value;
}

function escapeCell(value: string): string {
	const guarded = guardFormula(value);
	if (!/[",\n\r]/.test(guarded)) return guarded;
	return `"${guarded.replace(/"/g, '""')}"`;
}

/** Builds the per-VM activity CSV: one header row, then timestamp, actor,
 *  action and the plain-language label for each entry. */
export function buildActivityCsv(
	entries: readonly VmAuditEntry[],
	label: (action: string) => string
): string {
	const rows = entries.map((entry) =>
		[entry.timestamp, entry.actor, entry.action, label(entry.action)].map(escapeCell).join(',')
	);
	return [CSV_HEADER, ...rows].join('\r\n') + '\r\n';
}

/** The download name, e.g. `vm-100-activity-20260115.csv`. */
export function activityCsvFilename(vmid: number, now: Date = new Date()): string {
	const stamp = now.toISOString().slice(0, 10).replace(/-/g, '');
	return `vm-${vmid}-activity-${stamp}.csv`;
}
