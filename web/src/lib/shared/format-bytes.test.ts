import { describe, it, expect } from 'vitest';
import { formatBytes } from './format-bytes';

describe('formatBytes', () => {
	it('formats zero bytes', () => {
		expect(formatBytes(0)).toBe('0.0 B');
	});

	it('formats bytes without converting when smaller than a KiB', () => {
		expect(formatBytes(512)).toBe('512.0 B');
	});

	it('formats KiB', () => {
		expect(formatBytes(1536)).toBe('1.5 KiB');
	});

	it('formats MiB', () => {
		expect(formatBytes(1048576)).toBe('1.0 MiB');
	});

	it('formats GiB', () => {
		expect(formatBytes(2147483648)).toBe('2.0 GiB');
	});

	it('formats TiB', () => {
		expect(formatBytes(1099511627776)).toBe('1.0 TiB');
	});
});
