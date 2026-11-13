import type { VmAuditEntry } from './detail.svelte';
import type { ActivityEntry } from '#lib/features/activity/activity.svelte.js';

const FORMULA_PREFIXES = ['=', '+', '-', '@'] as const;
const CSV_HEADER = 'timestamp,actor,action,label';
const USER_CSV_HEADER = 'timestamp,cluster,vmid,machine,actor,action,label';

/** A cell that could be read as a formula by a spreadsheet gets a leading
 *  quote so the value stays text. */
function guardFormula(value: string): string {
	const isFormula = FORMULA_PREFIXES.some((prefix) => value.startsWith(prefix));
	return isFormula ? `'${value}` : value;
}

function escapeCell(value: string): string {
	const guarded = guardFormula(value);
	if (!/[",\n\r]/.test(guarded)) return guarded;
	return `"${guarded.replaceAll('"', '""')}"`;
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

/** Builds the cross-machine activity CSV for the /activity page. */
export function buildUserActivityCsv(
	entries: readonly ActivityEntry[],
	label: (action: string) => string
): string {
	const rows = entries.map((entry) =>
		[entry.timestamp, entry.cluster, String(entry.vmid), entry.machineName, entry.actor, entry.action, label(entry.action)]
			.map(escapeCell)
			.join(',')
	);
	return [USER_CSV_HEADER, ...rows].join('\r\n') + '\r\n';
}

const dateStamp = (now: Date): string => now.toISOString().slice(0, 10).replaceAll('-', '');

/** The download name, e.g. `vm-100-activity-20260115.csv`. */
export function activityCsvFilename(vmid: number, now: Date = new Date()): string {
	return `vm-${vmid}-activity-${dateStamp(now)}.csv`;
}

/** The /activity download name, e.g. `activity-20260115.csv`. */
export const userActivityCsvFilename = (now: Date = new Date()): string => `activity-${dateStamp(now)}.csv`;

/** Triggers a browser download of a CSV string. */
export function downloadCsv(csv: string, filename: string): void {
	const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
	const anchor = document.createElement('a');
	anchor.href = url;
	anchor.download = filename;
	document.body.appendChild(anchor);
	anchor.click();
	anchor.remove();
	URL.revokeObjectURL(url);
}
