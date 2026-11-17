import { describe, expect, it } from 'vitest';
import { tagTextColor } from './tag-colors';

describe('tagTextColor', () => {
	it('uses dark text on light backgrounds', () => {
		expect(tagTextColor('#f59e0b')).toBe('#171717');
		expect(tagTextColor('#ffff00')).toBe('#171717');
	});
	it('uses light text on dark backgrounds', () => {
		expect(tagTextColor('#4f46e5')).toBe('#fafafa');
		expect(tagTextColor('#000000')).toBe('#fafafa');
	});
	it('falls back to light text on invalid input', () => {
		expect(tagTextColor('red')).toBe('#fafafa');
	});
});
