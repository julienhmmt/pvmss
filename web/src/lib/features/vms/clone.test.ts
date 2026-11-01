import { describe, expect, it } from 'vitest';
import { suggestCopyName } from './clone';

describe('suggestCopyName', () => {
	it('suggests -copy when no copy exists', () => {
		expect(suggestCopyName('web-01')).toBe('web-01-copy');
	});

	it('moves to -copy-2 when -copy is taken', () => {
		expect(suggestCopyName('web-01', ['web-01-copy'])).toBe('web-01-copy-2');
	});

	it('skips every taken numbered copy', () => {
		expect(suggestCopyName('web-01', ['web-01-copy', 'web-01-copy-2', 'other'])).toBe('web-01-copy-3');
	});

	it('ignores unrelated taken names', () => {
		expect(suggestCopyName('web-01', ['db-01', 'web-01'])).toBe('web-01-copy');
	});
});
