import { describe, it, expect, afterEach } from 'vitest';
import { mount } from 'svelte';
import OsMark from './OsMark.svelte';

function tileFor(ostype: string, size?: 'sm' | 'lg'): Element {
	document.body.innerHTML = '';
	mount(OsMark, {
		target: document.body,
		props: size === undefined ? { ostype } : { ostype, size }
	});
	const tile = document.body.querySelector('span');
	if (tile === null) throw new Error('OsMark rendered no tile');
	return tile;
}

describe('OsMark', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('tints a Linux guest with the accent tone', () => {
		expect(tileFor('l26').className).toContain('bg-sidebar-accent');
	});

	it('tints a Windows guest with the success tone', () => {
		expect(tileFor('win11').className).toContain('bg-success-soft');
	});

	it('falls back to the muted tone for an unknown family', () => {
		expect(tileFor('').className).toContain('bg-muted');
		expect(tileFor('solaris').className).toContain('bg-muted');
	});

	it('draws a different glyph per family', () => {
		const linux = tileFor('l26').innerHTML;
		const windows = tileFor('win11').innerHTML;
		const other = tileFor('').innerHTML;
		expect(new Set([linux, windows, other]).size).toBe(3);
	});

	it('applies the small size by default', () => {
		const tile = tileFor('l26');
		expect(tile.className).toContain('h-[42px]');
		expect(tile.className).toContain('w-[38px]');
	});

	it('applies the large size when requested', () => {
		const tile = tileFor('l26', 'lg');
		expect(tile.className).toContain('h-[58px]');
		expect(tile.className).toContain('w-[53px]');
	});
});
