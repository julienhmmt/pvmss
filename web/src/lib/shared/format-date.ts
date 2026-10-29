import { getLocale } from '#lib/paraglide/runtime.js';

/** `short` for table cells, `medium` for detail views and feeds. */
export type DateStyle = 'short' | 'medium';

// Building an Intl formatter per row is measurable in long lists.
const formatters = new Map<string, Intl.DateTimeFormat>();

function formatter(options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
	const locale = getLocale();
	const key = `${locale}|${options.dateStyle ?? ''}|${options.timeStyle ?? ''}`;
	let cached = formatters.get(key);
	if (!cached) {
		cached = new Intl.DateTimeFormat(locale, options);
		formatters.set(key, cached);
	}
	return cached;
}

function format(value: string | Date, options: Intl.DateTimeFormatOptions): string {
	const date = value instanceof Date ? value : new Date(value);
	if (Number.isNaN(date.getTime())) return String(value);
	return formatter(options).format(date);
}

/** Date and time in the interface locale (not the browser's). */
export function formatDateTime(value: string | Date, style: DateStyle = 'medium'): string {
	return format(value, { dateStyle: style, timeStyle: 'short' });
}

/** Date only, in the interface locale. */
export function formatDate(value: string | Date): string {
	return format(value, { dateStyle: 'medium' });
}

/** Time only, in the interface locale. */
export function formatTime(value: string | Date): string {
	return format(value, { timeStyle: 'short' });
}

type FieldPart = 'year' | 'month' | 'day' | 'hour' | 'minute';
const FIELD_TOKENS: Record<FieldPart, string> = { year: 'yyyy', month: 'mm', day: 'dd', hour: 'hh', minute: 'mm' };

function dateTimeParts(): Intl.DateTimeFormatPart[] {
	return new Intl.DateTimeFormat(getLocale(), {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(new Date(2000, 0, 2, 3, 4));
}

/** Typing hint for a date-time text field in the interface locale, e.g. `mm/dd/yyyy, hh:mm` (en) or `dd/mm/yyyy hh:mm` (fr). */
export function dateTimeInputPattern(): string {
	return dateTimeParts()
		.map((p) => (p.type in FIELD_TOKENS ? FIELD_TOKENS[p.type as FieldPart] : p.value))
		.join('');
}

/**
 * Parses a date-time typed in the interface locale's field order
 * (see {@link dateTimeInputPattern}). The time is optional (midnight).
 * Returns null when the text is not a real date.
 */
export function parseDateTimeInput(text: string): Date | null {
	const order = dateTimeParts()
		.map((p) => p.type)
		.filter((t): t is FieldPart => t in FIELD_TOKENS);
	const numbers = text.match(/\d+/g) ?? [];
	if (numbers.length !== 3 && numbers.length !== 5) return null;
	const v: Record<FieldPart, number> = { year: 0, month: 0, day: 0, hour: 0, minute: 0 };
	order.slice(0, numbers.length).forEach((field, i) => (v[field] = Number(numbers[i])));
	const date = new Date(v.year, v.month - 1, v.day, v.hour, v.minute);
	const valid =
		v.year >= 1000 &&
		date.getFullYear() === v.year &&
		date.getMonth() === v.month - 1 &&
		date.getDate() === v.day &&
		date.getHours() === v.hour &&
		date.getMinutes() === v.minute;
	return valid ? date : null;
}
