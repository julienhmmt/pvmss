/** Curated tag hues that read well on both themes (Tailwind 500/600 shades). */
export const TAG_COLOR_PRESETS = [
	'#ef4444', '#f97316', '#f59e0b', '#84cc16', '#22c55e', '#14b8a6',
	'#06b6d4', '#3b82f6', '#4f46e5', '#8b5cf6', '#d946ef', '#64748b'
] as const;

export const HEX_COLOR = /^#[0-9a-fA-F]{6}$/;

const LIGHT_TEXT = '#fafafa';
const DARK_TEXT = '#171717';

function luminance(hex: string): number {
	const channel = (i: number) => {
		const c = parseInt(hex.slice(i, i + 2), 16) / 255;
		return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5);
}

const contrast = (a: number, b: number) => (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

/** Text colour with the higher WCAG contrast on a tag background (any admin-picked hex). */
export function tagTextColor(bg: string): string {
	if (!HEX_COLOR.test(bg)) return LIGHT_TEXT;
	const l = luminance(bg);
	return contrast(l, luminance(LIGHT_TEXT)) >= contrast(l, luminance(DARK_TEXT)) ? LIGHT_TEXT : DARK_TEXT;
}
