import { describe, it, expect, afterEach } from 'vitest';
import { mount } from 'svelte';
import OsMark from './OsMark.svelte';

describe('OsMark', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders the initials', () => {
		mount(OsMark, { target: document.body, props: { initials: 'Ub', tone: 'accent' } });
		expect(document.body.textContent).toContain('Ub');
	});

	it('applies the accent tone classes', () => {
		mount(OsMark, { target: document.body, props: { initials: 'Ub', tone: 'accent' } });
		const tile = document.body.querySelector('span');
		expect(tile!.className).toContain('bg-sidebar-accent');
	});

	it('applies the subtle tone classes', () => {
		mount(OsMark, { target: document.body, props: { initials: 'De', tone: 'subtle' } });
		const tile = document.body.querySelector('span');
		expect(tile!.className).toContain('bg-muted');
	});

	it('applies the success tone classes', () => {
		mount(OsMark, { target: document.body, props: { initials: 'Ro', tone: 'success' } });
		const tile = document.body.querySelector('span');
		expect(tile!.className).toContain('bg-success-soft');
	});

	it('applies the small size by default', () => {
		mount(OsMark, { target: document.body, props: { initials: 'Ub', tone: 'accent' } });
		const tile = document.body.querySelector('span');
		expect(tile!.className).toContain('h-[42px]');
		expect(tile!.className).toContain('w-[38px]');
	});

	it('applies the large size when requested', () => {
		mount(OsMark, {
			target: document.body,
			props: { initials: 'Ub', tone: 'accent', size: 'lg' }
		});
		const tile = document.body.querySelector('span');
		expect(tile!.className).toContain('h-[58px]');
		expect(tile!.className).toContain('w-[53px]');
	});
});
