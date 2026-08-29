import { describe, expect, it } from 'vitest';
import { formatBytes } from './format';

describe('formatBytes', () => {
	it('formats bytes below 1 MB as bytes', () => {
		expect(formatBytes(0)).toBe('0 B');
		expect(formatBytes(512)).toBe('512 B');
		expect(formatBytes(999999)).toBe('999999 B');
	});

	it('formats bytes between 1 MB and 1 GB as MB', () => {
		expect(formatBytes(1e6)).toBe('1 MB');
		expect(formatBytes(5e6)).toBe('5 MB');
		expect(formatBytes(999999999)).toBe('1000 MB');
	});

	it('formats bytes at or above 1 GB as GB with one decimal', () => {
		expect(formatBytes(1e9)).toBe('1.0 GB');
		expect(formatBytes(1.5e9)).toBe('1.5 GB');
		expect(formatBytes(10e9)).toBe('10.0 GB');
	});
});
