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
