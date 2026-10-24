import { describe, it, expect } from 'vitest';
import { osFamily } from './os-family';

describe('osFamily', () => {
	it('maps Proxmox Linux kernel types to linux', () => {
		expect(osFamily('l26')).toBe('linux');
		expect(osFamily('l24')).toBe('linux');
	});

	it('maps every Windows family, including the ones not prefixed "win"', () => {
		for (const ostype of ['wxp', 'w2k', 'w2k3', 'w2k8', 'wvista', 'win7', 'win8', 'win10', 'win11']) {
			expect(osFamily(ostype)).toBe('windows');
		}
	});

	it('never guesses a family it cannot know', () => {
		// `other` is a real Proxmox value; `solaris` is not Linux or Windows;
		// an empty string means the config reported nothing.
		for (const ostype of ['other', 'solaris', '', 'l', 'win']) {
			expect(osFamily(ostype)).toBe('other');
		}
	});

	it('is case- and whitespace-insensitive', () => {
		expect(osFamily(' L26 ')).toBe('linux');
		expect(osFamily('WIN11')).toBe('windows');
	});
});
